#!/usr/bin/env bash
# cleanup-dev-users.sh — уборка тестовых сотрудников и их проектов в dev-окружении.
#
# Зачем: прогоны e2e оставляют после себя сотни сотрудников (у каждого своя
# запись в users и свой tenant), и через месяц в «Команде» невозможно понять,
# кто настоящий. Скрипт удаляет мусор и НИЧЕГО не делает с реальными людьми.
#
# Что считается мусором: домены example.com, test.dev, demo.local, user.ru —
# те, что не могут быть настоящими в компании. Настоящий admin@admin.ru и его
# проекты не трогаются.
#
# Почему нельзя просто удалить строки: на сотрудника ссылаются таблицы с
# ON DELETE NO ACTION (аудит, расчёты, платежи, выдача прав, автор настроек).
# Поэтому сначала эти ссылки обнуляются (SET NULL — сам PostgreSQL так и
# предлагает), потом удаляются проекты, и только потом сам человек. История
# при этом не переписывается молча: пропадает лишь «кто именно автор», у
# тестовых записей это и не ценность.
#
# Режимы:
#   (по умолчанию) — dry-run: только показывает, что будет удалено;
#   --apply        — выполнить;
#   --keep-projects — не удалять проекты, а только людей (проекты останутся
#                     без владельца, поэтому допустим только для базы, где
#                     проектов тестовых нет);
#   --email-contains=— чистить только совпадения по подстроке почты.
#
# Переменные:
#   STAIR_DATABASE_URL  строка подключения (по умолчанию — docker-compose);
#   PG_CONTAINER        контейнер PostgreSQL для docker-режима;
#   PG_DOCKER_MODE      auto|always|never (по умолчанию auto).
set -euo pipefail

STAIR_DATABASE_URL="${STAIR_DATABASE_URL:-postgres://stair:changeme@127.0.0.1:5432/stair_platform?sslmode=disable}"
PG_CONTAINER="${PG_CONTAINER:-stair-platform-postgres}"
PG_DOCKER_MODE="${PG_DOCKER_MODE:-auto}"
APPLY=0
KEEP_PROJECTS=0
EMAIL_FILTER=""

for arg in "$@"; do
	case "$arg" in
	--apply) APPLY=1 ;;
	--keep-projects) KEEP_PROJECTS=1 ;;
	--email-contains=*) EMAIL_FILTER="${arg#*=}" ;;
	-h | --help)
		sed -n '2,32p' "$0"
		exit 0
		;;
	*)
		echo "cleanup-dev-users: неизвестный аргумент $arg" >&2
		exit 2
		;;
	esac
done

url_no_params="${STAIR_DATABASE_URL%%\?*}"
DB_NAME="${url_no_params##*/}"

# Режим доступа к psql — как в db-backup.sh: локально ходим внутрь контейнера,
# в CI используем свой psql и STAIR_DATABASE_URL.
psql_cmd() {
	if [ "$PG_DOCKER_MODE" != "never" ] &&
		{ [ "$PG_DOCKER_MODE" = "always" ] || ! command -v psql >/dev/null 2>&1; }; then
		docker exec -i "$PG_CONTAINER" psql -v ON_ERROR_STOP=1 -U "${PG_USER:-stair}" -d "$DB_NAME" "$@"
	else
		psql -v ON_ERROR_STOP=1 "$STAIR_DATABASE_URL" "$@"
	fi
}

# Условие «это мусор». Один список на двух языках — скрипт и отчёт обязаны
# считать одно и то же, иначе отчёт врёт.
read -r -d '' JUNK_WHERE <<SQL || true
email LIKE '%@example.com' OR email LIKE '%@test.dev' OR email LIKE '%@demo.local' OR email LIKE '%@user.ru'
SQL

if [ -n "$EMAIL_FILTER" ]; then
	JUNK_WHERE="email LIKE '%${EMAIL_FILTER}%'"
fi

echo "== кандидаты на удаление =="
psql_cmd -tAc "SELECT count(*) FROM users WHERE deleted_at IS NULL AND (${JUNK_WHERE})" |
	sed 's/^/  сотрудников: /'
psql_cmd -tAc "SELECT count(*) FROM projects p JOIN users u ON u.id = p.owner_id WHERE u.deleted_at IS NULL AND (${JUNK_WHERE})" |
	sed 's/^/  их проектов: /'
psql_cmd -tAc "SELECT count(*) FROM tenants t WHERE EXISTS (SELECT 1 FROM users u WHERE u.tenant_id = t.id AND (${JUNK_WHERE}))" |
	sed 's/^/  их тенантов: /'

if [ "$APPLY" -ne 1 ]; then
	echo
	echo "Это был пробный прогон. Ничего не изменено. Повторите с --apply, когда проверите цифры."
	exit 0
fi

if [ "$KEEP_PROJECTS" -ne 1 ]; then
	echo "== удаляю проекты тестовых сотрудников =="
	psql_cmd -c "
		DELETE FROM projects p
		USING users u
		WHERE p.owner_id = u.id AND u.deleted_at IS NULL AND (${JUNK_WHERE})"
else
	echo "== проекты оставлены (--keep-projects) =="
fi

# Ссылок с ON DELETE NO ACTION, но nullable: обнуляем автора, чтобы человек
# удалился, а записи (аудит, расчёты, платежи, настройки) остались.
#
# Список жертв кладём во временную таблицу, а не в CTE: CTE живёт только один
# запрос, и следующие UPDATE-ы не видели бы её — ошибка выглядела бы как
# «первая таблица обновилась, дальше тишина».
echo "== обнуляю ссылки на удаляемых сотрудников =="
psql_cmd <<SQL
BEGIN;
CREATE TEMP TABLE victims (id uuid PRIMARY KEY) ON COMMIT DROP;
INSERT INTO victims (id) SELECT id FROM users WHERE deleted_at IS NULL AND (${JUNK_WHERE});
UPDATE audit_events SET actor_id = NULL WHERE actor_id IN (SELECT id FROM victims);
UPDATE api_keys SET created_by = NULL WHERE created_by IN (SELECT id FROM victims);
UPDATE calc_jobs SET user_id = NULL WHERE user_id IN (SELECT id FROM victims);
UPDATE payment_intents SET user_id = NULL WHERE user_id IN (SELECT id FROM victims);
UPDATE store_rates SET updated_by = NULL WHERE updated_by IN (SELECT id FROM victims);
UPDATE store_settings SET updated_by = NULL WHERE updated_by IN (SELECT id FROM victims);
UPDATE user_grants SET granted_by = NULL WHERE granted_by IN (SELECT id FROM victims);
COMMIT;
SQL

echo "== удаляю сотрудников =="
psql_cmd <<SQL
BEGIN;
CREATE TEMP TABLE victims (id uuid PRIMARY KEY) ON COMMIT DROP;
INSERT INTO victims (id) SELECT id FROM users WHERE deleted_at IS NULL AND (${JUNK_WHERE});
DELETE FROM users WHERE id IN (SELECT id FROM victims);
COMMIT;
SQL

# Тенанты остаются сиротами (на них же нет внешнего ключа с каскадом), чистим
# явно — иначе список тенантов в dev будет расти без причины.
echo "== удаляю пустые тенанты тестовых сотрудников =="
psql_cmd -c "
	DELETE FROM tenants t
	WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.tenant_id = t.id)
	  AND t.slug LIKE 't-%'"

echo
echo "Готово. Осталось:"
psql_cmd -tAc "SELECT count(*) FROM users" | sed 's/^/  всего сотрудников: /'
psql_cmd -tAc "SELECT count(*) FROM projects" | sed 's/^/  всего проектов: /'
