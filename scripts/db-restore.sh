#!/usr/bin/env bash
# db-restore.sh — восстановление PostgreSQL из custom-дампа db-backup.sh.
#
# Использование:
#   scripts/db-restore.sh <dump-file> [target-db]
#
# Переменные:
#   STAIR_DATABASE_URL  строка подключения (для host-режима и admin-операций)
#   TARGET_DB           целевая БД (2-й аргумент имеет приоритет); по умолчанию <source>_restored
#   FORCE=1             разрешить восстановление поверх исходной БД
#   PG_CONTAINER        контейнер для docker-режима (default stair-platform-postgres)
#   PG_DOCKER_MODE      auto|always|never (default auto)
#
# Перед восстановлением проверяется checksum (если рядом есть <dump>.sha256).
set -euo pipefail

DUMP="${1:-}"
TARGET_DB="${2:-${TARGET_DB:-}}"
FORCE="${FORCE:-0}"
STAIR_DATABASE_URL="${STAIR_DATABASE_URL:-postgres://stair:stair@127.0.0.1:5432/stair_platform?sslmode=disable}"
PG_CONTAINER="${PG_CONTAINER:-stair-platform-postgres}"
PG_DOCKER_MODE="${PG_DOCKER_MODE:-auto}"

[ -n "$DUMP" ] || {
	echo "usage: scripts/db-restore.sh <dump-file> [target-db]" >&2
	exit 2
}
[ -f "$DUMP" ] || {
	echo "db-restore: dump not found: $DUMP" >&2
	exit 1
}

url_no_params="${STAIR_DATABASE_URL%%\?*}"
SRC_DB="${url_no_params##*/}"
userinfo="${STAIR_DATABASE_URL#*://}"
userinfo="${userinfo%%@*}"
DB_USER="${userinfo%%:*}"
DB_PASS=""
if [ "$userinfo" != "${userinfo%%:*}" ]; then
	DB_PASS="${userinfo#*:}"
fi

q=""
case "$STAIR_DATABASE_URL" in
*\?*) q="?${STAIR_DATABASE_URL#*\?}" ;;
esac
base="${STAIR_DATABASE_URL%%\?*}"
base="${base%/*}"

if [ -z "$TARGET_DB" ]; then
	META_DB=""
	if [ -f "$DUMP.meta" ]; then
		META_DB="$(sed -n 's/^database: //p' "$DUMP.meta")"
	fi
	TARGET_DB="${META_DB:-$SRC_DB}_restored"
fi

if [ "$TARGET_DB" = "$SRC_DB" ] && [ "$FORCE" != "1" ]; then
	echo "db-restore: refusing to restore over source database '$SRC_DB' (set FORCE=1 to override)" >&2
	exit 1
fi

use_docker() {
	case "$PG_DOCKER_MODE" in
	always) return 0 ;;
	never) return 1 ;;
	esac
	if command -v pg_restore >/dev/null 2>&1; then
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

if [ -f "$DUMP.sha256" ]; then
	expected="$(awk '{print $1}' "$DUMP.sha256")"
	actual="$(sha256_of "$DUMP")"
	if [ "$expected" != "$actual" ]; then
		echo "db-restore: checksum mismatch for $DUMP" >&2
		echo "  expected: $expected" >&2
		echo "  actual:   $actual" >&2
		exit 1
	fi
	echo "db-restore: checksum OK ($actual)"
fi

if use_docker; then
	exists="$(docker exec "$PG_CONTAINER" psql -U "$DB_USER" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='$TARGET_DB'")"
	if [ "$exists" != "1" ]; then
		docker exec "$PG_CONTAINER" psql -U "$DB_USER" -d postgres -c "CREATE DATABASE \"$TARGET_DB\"" >/dev/null
	fi
	echo "db-restore: restoring $DUMP -> '$TARGET_DB' (docker)"
	docker exec -i "$PG_CONTAINER" pg_restore --clean --if-exists --no-owner --no-privileges -U "$DB_USER" -d "$TARGET_DB" <"$DUMP"
	tables="$(docker exec "$PG_CONTAINER" psql -U "$DB_USER" -d "$TARGET_DB" -tAc "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'")"
else
	exists="$(PGPASSWORD="$DB_PASS" psql "$base/postgres$q" -tAc "SELECT 1 FROM pg_database WHERE datname='$TARGET_DB'")"
	if [ "$exists" != "1" ]; then
		PGPASSWORD="$DB_PASS" psql "$base/postgres$q" -c "CREATE DATABASE \"$TARGET_DB\"" >/dev/null
	fi
	echo "db-restore: restoring $DUMP -> '$TARGET_DB' (host)"
	PGPASSWORD="$DB_PASS" pg_restore --clean --if-exists --no-owner --no-privileges -d "$base/$TARGET_DB$q" "$DUMP"
	tables="$(PGPASSWORD="$DB_PASS" psql "$base/$TARGET_DB$q" -tAc "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'")"
fi

echo "db-restore: OK"
echo "  target database: $TARGET_DB"
echo "  public tables:   $tables"
