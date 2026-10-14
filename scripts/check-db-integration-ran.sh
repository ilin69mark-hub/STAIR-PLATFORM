#!/usr/bin/env bash
# STAIR PLATFORM — гейт «БД-интеграция действительно выполнялась» (CRITICAL-01/05).
#
# ЗАЧЕМ. Интеграционные тесты internal/infrastructure/database пропускают себя
# через `t.Skip`, если не задана STAIR_TEST_DATABASE_URL. Это удобно локально,
# но в CI означает ловушку: пакет целиком скипается, `go test ./...` зелёный,
# а джоба радостно рапортует об успехе. Именно так в репозитории проскочили
# дефекты, найденные 2026-09-27:
#
#   * `TestConcurrentConfigurationRevision` падал на CHECK
#     stair_configurations_positive_geometry (неполная фикстура);
#   * `TestAuditListForeignTenant` падал на «unknown result ""»;
#   * `TestPublicQuoteUsesStoreRates` проходил вхолостую, потому что тестовый
#     роутер не был собран как прод.
#
# Первая из причин — миграция 000031 с `comfort_step_mm > 0` ломала `make
# seed` и любое сохранение конфигурации без необязательных полей. Всё это
# было зелёным именно потому, что тесты не запускались.
#
# Сценарий повторного пропуска (например, кто-то уберёт env из джобы, сервис
# postgres не поднимется, сменится имя переменной) обязан валить джобу, а не
# тихо зеленеть. Этот скрипт — противоядие: он требует, чтобы конкретные
# тесты-«маяки» реально выполнились.
#
# Usage:
#   scripts/check-db-integration-ran.sh
#   STAIR_TEST_DATABASE_URL=postgres://... scripts/check-db-integration-ran.sh

set -euo pipefail

DB_PKG="./internal/infrastructure/database"

if [[ -z "${STAIR_TEST_DATABASE_URL:-}" ]]; then
  echo "ERROR: STAIR_TEST_DATABASE_URL is not set."
  echo "       Без него весь $DB_PKG скипается, и гейт ничем не отличается от провала."
  exit 1
fi

# Тесты-маяки. По одному на каждый закрытый CRITICAL этого цикла: если какой-то
# из них не выполнился, значит либо БД недоступна, либо файл теста потерян —
# и то, и другое обязано быть видно, а не замаскировано зелёной джобой.
#
# Имена указаны явно, без «grep по всему пакету»: пропуск легитимных
# (TestWithTx* требует настройки serializable-изоляции, TestVectorSearchRanking
# — pgvector) не должен ронять гейт.
SENTINELS=(
  # CRITICAL-01: геометрический CHECK допускает легальные нули (make seed).
  TestSaveConfigWithZeroOptionalGeometrySucceeds
  # DB-001: конкурентные ревизии конфигурации.
  TestConcurrentConfigurationRevision
  # CRITICAL-04: платёжный журнал не пишет несуществующие события.
  TestCRIT004_RejectedTransitionWritesNoEvent
  # CRITICAL-05: атомарная блокировка закрывает ключи и сессии.
  TestCRIT005_DisableUserClosesBothEntryPoints
  # Миграции применяются целиком, включая последнюю.
  TestMigrateToVersionIntegration
  # Аудит-репозиторий: tenant-скоуп.
  TestAuditListForeignTenant
)

echo "Running database integration tests to verify they actually execute..."
OUT="$(go test -v -count=1 -p 1 "$DB_PKG" 2>&1)" && STATUS=0 || STATUS=$?

FAILED=0

# 1. Скип по отсутствию БД — самый частый и самый опасный случай.
if grep -q "STAIR_TEST_DATABASE_URL not set" <<<"$OUT"; then
  echo "ERROR: integration tests were SKIPPED — the database is unreachable."
  grep -n "STAIR_TEST_DATABASE_URL not set" <<<"$OUT" | head -5
  FAILED=1
fi

# 2. Пакет не собрался или упал раньше, чем дошли до сентинелов.
if [[ $STATUS -ne 0 ]]; then
  echo "ERROR: go test exited with $STATUS before the gate could verify sentinels."
  echo "$OUT" | tail -40
  exit 1
fi

# 3. Каждый маяк обязан быть выполнен (--- PASS на верхнем уровне).
for t in "${SENTINELS[@]}"; do
  if grep -qE "^--- PASS: ${t}([[:space:]]|$)" <<<"$OUT"; then
    echo "  ok   $t"
  else
    echo "  MISS $t — тест не выполнился"
    FAILED=1
  fi
done

if [[ $FAILED -ne 0 ]]; then
  echo
  echo "Gate FAILED: БД-интеграция не даёт доказательства выполнения."
  echo "Джоба проходить не должна: молчаливо скипанная интеграция — это и есть"
  echo "причина, по которой CRITICAL-01/04/05 держались зелёными."
  exit 1
fi

echo
echo "Gate PASSED: ${#SENTINELS[@]} тестов- маяков выполнены на живой БД."
