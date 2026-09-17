#!/usr/bin/env bash
# db-backup-restore-check.sh — проверка обратимости резервного копирования:
# backup -> restore в отдельную БД -> сверка схемы и данных -> cleanup.
#
# Переменные:
#   STAIR_DATABASE_URL  исходная БД (по умолчанию docker-compose)
#   CHECK_DB            имя проверочной БД (default <source>_restore_check)
#   PG_CONTAINER        контейнер PostgreSQL (default stair-platform-postgres)
set -euo pipefail

STAIR_DATABASE_URL="${STAIR_DATABASE_URL:-postgres://stair:stair@127.0.0.1:5432/stair_platform?sslmode=disable}"
PG_CONTAINER="${PG_CONTAINER:-stair-platform-postgres}"

url_no_params="${STAIR_DATABASE_URL%%\?*}"
SRC_DB="${url_no_params##*/}"
CHECK_DB="${CHECK_DB:-${SRC_DB}_restore_check}"
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

use_docker=0
if ! command -v psql >/dev/null 2>&1; then
	use_docker=1
fi

TMP_DIR="$(mktemp -d)"
cleanup() {
	if [ "$use_docker" = "1" ]; then
		docker exec "$PG_CONTAINER" psql -U "$DB_USER" -d postgres -c "DROP DATABASE IF EXISTS \"$CHECK_DB\"" >/dev/null 2>&1 || true
	else
		PGPASSWORD="$DB_PASS" psql "$base/postgres$q" -c "DROP DATABASE IF EXISTS \"$CHECK_DB\"" >/dev/null 2>&1 || true
	fi
	rm -rf "$TMP_DIR"
}
trap cleanup EXIT

psql_q() { # $1=db, $2=sql
	if [ "$use_docker" = "1" ]; then
		docker exec "$PG_CONTAINER" psql -U "$DB_USER" -d "$1" -tAc "$2"
	else
		PGPASSWORD="$DB_PASS" psql "$base/$1$q" -tAc "$2"
	fi
}

echo "== db-backup-restore-check: source='$SRC_DB' check='$CHECK_DB' =="

BACKUP_DIR="$TMP_DIR" ./scripts/db-backup.sh >/dev/null
DUMP="$(find "$TMP_DIR" -name '*.dump' | head -1)"
[ -n "$DUMP" ] || {
	echo "check: backup did not produce a dump" >&2
	exit 1
}

./scripts/db-restore.sh "$DUMP" "$CHECK_DB" >/dev/null

src_tables="$(psql_q "$SRC_DB" "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'")"
chk_tables="$(psql_q "$CHECK_DB" "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'")"
echo "tables: source=$src_tables restored=$chk_tables"
[ "$src_tables" = "$chk_tables" ] || {
	echo "check: table count mismatch" >&2
	exit 1
}

for tbl in schema_migrations schema_migrations_seeds users projects; do
	has="$(psql_q "$SRC_DB" "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name='$tbl'")"
	[ "$has" = "1" ] || continue
	s="$(psql_q "$SRC_DB" "SELECT count(*) FROM $tbl")"
	r="$(psql_q "$CHECK_DB" "SELECT count(*) FROM $tbl")"
	echo "rows: $tbl source=$s restored=$r"
	[ "$s" = "$r" ] || {
		echo "check: row count mismatch in $tbl" >&2
		exit 1
	}
done

echo "db-backup-restore-check: OK (backup and restore are reversible)"
