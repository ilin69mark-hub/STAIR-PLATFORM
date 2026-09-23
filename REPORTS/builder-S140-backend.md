# S-140-backend: Sentry-интеграция Go-бэкенда

**Вердикт:** DONE — SDK инициализируется только по `STAIR_SENTRY_DSN` (пусто = no-op), паники уходят из recovery-middleware, PII-скраб (всё + хеш email) через BeforeSend, полный сьют зелёный. Коммит в `dev/swarm/ws-s132b-client` (не запушен). Реальный DSN вносится человеком позже (S-118 отложен).

## Что сделано

- **Новый `internal/infrastructure/sentry/`**: `sentry.go` — `Init(Config)` (DSN пусто = безопасный no-op, shutdown флашит буфер), `CapturePanic(recovered, r, stack)` (no-op без клиента); `scrub.go` — `BeforeSend`-хук: JSON round-trip всего события через `redaction.Sensitive`, хеш email (SHA-256:12, lowercase+trim) под любыми `*email*`-ключами, дроп IP/username/name, дроп заголовков Authorization/Cookie/X-Forwarded-For и др. + чистка query/data/cookies/URL; `sentry_test.go` — recordingTransport (без сети), табличные кейсы скраббера, init no-op, end-to-end capture→transport.
- **Новый `internal/infrastructure/redaction/`**: `redactSensitive` из `debug_log.go` (S-111) вынесен в общий пакет без изменения списка ключей; `debug_log.go` переведён на него (старые тесты赤字 перенесены в `redaction_test.go`).
- **Wiring**: `cmd/api/main.go` (service `stair-platform`) + `cmd/worker/main.go` (service `stair-platform-worker`) — init по `STAIR_SENTRY_DSN`, environment (`STAIR_ENVIRONMENT`, api default development), release = `version.Version`, `TracesSampleRate` из `STAIR_SENTRY_TRACES_SAMPLE_RATE` (default 0 — только ошибки; tracing остаётся за OTel). Ошибка init → лог + exit 1 (fail-fast, как S-104).
- **Recovery**: `internal/transport/http/recovery.go` — паника → лог + `sentry.CapturePanic` с контекстом запроса (метод/путь/стек в extra).
- **Доки**: `docs/17_INFRASTRUCTURE/17_OBSERVABILITY_INFRASTRUCTURE.md` — раздел Sentry (где задать DSN: GH Secrets → Helm/env; что скраббится; lazy-фронт — см. `REPORTS/frontend-S140.md`).
- Зависимость: `github.com/getsentry/sentry-go` (стабильная) в go.mod/go.sum.

## Гейты

- `go build ./...` — ok; `gofmt` — чисто; `go vet` (sentry/redaction/http/cmd) — 0; `golangci-lint run` на затронутых пакетах — **0 issues**.
- Пакетные: `sentry`, `redaction`, `transport/http` — ok (race).
- Полный `STAIR_TEST_DATABASE_URL=... go test -race -p 1 -count=1 -timeout 25m ./...` — **60/60 ok, 0 FAIL** (было 58 пакетов, +2 новых: sentry, redaction). БД после: version=24, dirty=false.
- Фронт S-140 (коммит `20fc105`, @frontend-dev) не тронут; его файлы в этот коммит не входят.

## Открыто (за человеком)

- `STAIR_SENTRY_DSN` (Sentry cloud проект) → GH Secrets → Helm/env (S-118 отложен); фронт `VITE_SENTRY_DSN` + `SENTRY_AUTH_TOKEN`/ORG/PROJECT для sourcemaps.
- После внесения DSN: один тестовый паник-эндпоинт на staging → проверить событие в Sentry-панели (скраб глазками).
