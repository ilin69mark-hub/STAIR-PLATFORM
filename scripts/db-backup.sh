#!/usr/bin/env bash
# db-backup.sh — резервная копия PostgreSQL в custom-формате (+ checksum + метаданные).
# AUDIT-EXCEPTION(E11): GPG-шифрование дампов — человек, см. docs/SECURITY_EXCEPTIONS.yml
#
# Режимы доступа:
#   - host:  если в PATH есть pg_dump — используется он и STAIR_DATABASE_URL;
#   - docker: иначе pg_dump выполняется внутри контейнера PG_CONTAINER.
#
# Переменные:
#   STAIR_DATABASE_URL  строка подключения (по умолчанию — docker-compose)
#   BACKUP_DIR          каталог для копий (по умолчанию ./backups)
#   PG_CONTAINER        имя контейнера для docker-режима (default stair-platform-postgres)
#   PG_DOCKER_MODE      auto|always|never (default auto)
set -euo pipefail

STAIR_DATABASE_URL="${STAIR_DATABASE_URL:-postgres://stair:changeme@127.0.0.1:5432/stair_platform?sslmode=disable}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
PG_CONTAINER="${PG_CONTAINER:-stair-platform-postgres}"
PG_DOCKER_MODE="${PG_DOCKER_MODE:-auto}"
# S-128: сколько дней хранить дампы; 0 — не удалять старые.
BACKUP_RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}"

url_no_params="${STAIR_DATABASE_URL%%\?*}"
DB_NAME="${url_no_params##*/}"
[ -n "$DB_NAME" ] && [ "$DB_NAME" != "$url_no_params" ] || {
	echo "db-backup: cannot parse database name from STAIR_DATABASE_URL" >&2
	exit 1
}

userinfo="${STAIR_DATABASE_URL#*://}"
userinfo="${userinfo%%@*}"
DB_USER="${userinfo%%:*}"
DB_PASS=""
if [ "$userinfo" != "${userinfo%%:*}" ]; then
	DB_PASS="${userinfo#*:}"
fi

use_docker() {
	case "$PG_DOCKER_MODE" in
	always) return 0 ;;
	never) return 1 ;;
	esac
	if command -v pg_dump >/dev/null 2>&1; then
		return 1
	fi
	docker inspect "$PG_CONTAINER" >/dev/null 2>&1
}

sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	else
		shasum -a 256 "$1" | awk '{print $1}'
	fi
}

pg_version() {
	if use_docker; then
		docker exec "$PG_CONTAINER" pg_dump --version
	else
		pg_dump --version
	fi
}

TS="$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$BACKUP_DIR"
OUT="$BACKUP_DIR/${DB_NAME}_${TS}.dump"
TMP="$OUT.tmp"

if use_docker; then
	echo "db-backup: dumping '$DB_NAME' via docker container '$PG_CONTAINER' -> $OUT"
	docker exec "$PG_CONTAINER" pg_dump -Fc --no-owner --no-privileges -U "$DB_USER" -d "$DB_NAME" >"$TMP"
else
	echo "db-backup: dumping '$DB_NAME' via host pg_dump -> $OUT"
	PGPASSWORD="$DB_PASS" pg_dump -Fc --no-owner --no-privileges -d "$STAIR_DATABASE_URL" >"$TMP"
fi

[ -s "$TMP" ] || {
	rm -f "$TMP"
	echo "db-backup: dump is empty or failed" >&2
	exit 1
}
mv "$TMP" "$OUT"

SUM="$(sha256_of "$OUT")"
printf '%s  %s\n' "$SUM" "$(basename "$OUT")" >"$OUT.sha256"

SIZE="$(wc -c <"$OUT" | tr -d ' ')"
{
	printf 'database: %s\n' "$DB_NAME"
	printf 'user: %s\n' "$DB_USER"
	printf 'created_utc: %s\n' "$TS"
	printf 'dump_format: custom (pg_dump -Fc)\n'
	printf 'size_bytes: %s\n' "$SIZE"
	printf 'sha256: %s\n' "$SUM"
	printf 'pg_version: %s\n' "$(pg_version)"
} >"$OUT.meta"

echo "db-backup: OK"
echo "  dump:     $OUT"
echo "  checksum: $OUT.sha256 ($SUM)"
echo "  metadata: $OUT.meta"

# S-128: retention — удаляем дампы старше BACKUP_RETENTION_DAYS (только
# файлы вида <db>_*.dump + их .sha256/.meta; 0 — пропуск).
if [ "$BACKUP_RETENTION_DAYS" != "0" ]; then
	# Некорректное значение — предупреждаем и пропускаем (бэкап уже готов).
	case "$BACKUP_RETENTION_DAYS" in
	'' | *[!0-9]*)
		echo "db-backup: WARNING: bad BACKUP_RETENTION_DAYS='$BACKUP_RETENTION_DAYS', skipping prune" >&2
		;;
	*)
		PRUNED=0
		while IFS= read -r old; do
			[ -n "$old" ] || continue
			rm -f "$old" "$old.sha256" "$old.meta"
			PRUNED=$((PRUNED + 1))
		done <<EOF
$(find "$BACKUP_DIR" -maxdepth 1 -name "${DB_NAME}_*.dump" -mtime +"$BACKUP_RETENTION_DAYS" -print)
EOF
		echo "  retention: pruned $PRUNED dump(s) older than $BACKUP_RETENTION_DAYS day(s)"
		;;
	esac
fi
