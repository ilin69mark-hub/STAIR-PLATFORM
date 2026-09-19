#!/usr/bin/env bash
# STAIR PLATFORM — coverage quality gate (DEV-0010 / TEST-0028).
#
# Проверяет, что суммарное покрытие тестами не ниже порога.
# Пакет infrastructure/database требует БД (STAIR_TEST_DATABASE_URL):
# если переменная не задана, интеграционные тесты в нём скипаются и
# покрытие равно 0% — такой пакет исключается из подсчёта, чтобы гейт
# работал одинаково локально и в CI (без БД).
#
# Usage:
#   scripts/coverage-check.sh [threshold]
#   COVERAGE_THRESHOLD=85 scripts/coverage-check.sh
#   COVERAGE_PROFILE=coverage.out scripts/coverage-check.sh 85   # reuse готового профиля (CI: без повторного go test)

set -euo pipefail

THRESHOLD="${COVERAGE_THRESHOLD:-${1:-85}}"
DB_PKG="internal/infrastructure/database"

if [[ -n "${COVERAGE_PROFILE:-}" ]]; then
    profile="$COVERAGE_PROFILE"
    trap - EXIT
else
    profile="$(mktemp)"
    trap 'rm -f "$profile"' EXIT
    go test ./... -coverprofile="$profile" >/dev/null
fi

# Исключаем database-пакет, если тестовая БД не задана.
if [[ -z "${STAIR_TEST_DATABASE_URL:-}" ]]; then
    nodb="$(mktemp)"
    trap 'rm -f "$nodb"' EXIT
    grep -v "$DB_PKG/" "$profile" > "$nodb" || true
    profile="$nodb"
fi

total="$(go tool cover -func="$profile" | awk '/total:/{gsub(/%/,"",$NF); print $NF}')"

echo "total coverage: ${total}% (threshold ${THRESHOLD}%)"
if awk -v t="$total" -v th="$THRESHOLD" 'BEGIN{exit !(t < th)}'; then
    echo "ERROR: coverage below threshold" >&2
    exit 1
fi
