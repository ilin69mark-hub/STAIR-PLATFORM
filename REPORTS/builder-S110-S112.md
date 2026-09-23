# Builder — W5 (S-110) + W6 (S-112)

- **Ветка:** `dev/swarm/security-w5-w6` (от main `9c51bda`)
- **Коммиты:** `2118769` (W5 S-110) · `5fd7b65` (W6 S-112) — каждый компилируется и проходит тесты независимо
- **Зона:** internal/, cmd/, pkg/ — фронтенды, deployments, .github не тронуты

**Вердикт:** обе карточки реализованы: `?token=` в WS-запросах отклоняется (401) с аутентификацией только через cookie/Bearer, dead ByIP/XFF-код rate-limit удалён, WS-ориджины берутся из конфига с same-origin дефолтом — все DoD-гейты зелёные (включая `go test -race -p 1` с postgres:16).

## Что изменено

### W5 — S-110 [P2] WS-TOKEN-IN-QUERY
- `internal/transport/http/websocket_handler.go`:
  - query-параметр `?token=` намеренно отклоняется явным `401` (логируем Warn, токен в URL не парсится), даже если рядом валидная cookie — практика не может «незаметно сохраниться»;
  - аутентификация — только `Authorization: Bearer <token>` (не-браузерные клиенты) или session-cookie (`session`/`session_admin` по `X-App-Origin`, браузер шлёт её на same-origin WS-handshake автоматически, через существующий `sessionToken()`);
  - строка из fingerprint'а `websocket_handler.go:56` в отчёте S-103 закрыта.
- `internal/transport/http/websocket_handler_test.go`: 5 новых тестов — `RejectsQueryToken`, `RejectsQueryTokenEvenWithCookie`, `WithSessionCookie`, `WithAdminSessionCookie`, `InvalidSessionCookie`; существующие переведены с `?token=` на Bearer.
- `internal/transport/http/integration_test.go`: `TestWebSocketHandlerIntegration` — Bearer вместо `?token=`.
- **Фронтенды:** `frontend/` и `frontend-store/` **не используют WebSocket вообще** (grep по `WebSocket|ws://|wss://|/ws` = 0) — правки фронтов не требуются, ломать нечего.

### W6 — S-112 [P3]
**Таргет 1 — RATELIMIT-DEAD-XFF-CODE**
- `internal/infrastructure/security/ratelimit.go`: удалены `ByIP`/`ByEndpoint`/`ByUser`/`ByUserEndpoint` и `ctxUserID`/`userIDCtxKey` — мёртвые функции доверяли подделываемым `X-Forwarded-For`/`X-Real-IP` (латентный байпас). Live-путь не трогался: `clientIP()` в `middleware_auth.go` уже RemoteAddr-only (EDR-0014 §3.2.1, план trust-прокси из env-списка).
- `internal/transport/http/middleware_test.go`: регресс-тест `TestClientIPUsesRemoteAddrOnly` — XFF/X-Real-IP спуфинг игнорируется.
- `internal/infrastructure/security/ratelimit_test.go`: тесты удалённых функций сняты, `RateLimit`-тесты переведены на `remoteAddrKey` (t.R.RemoteAddr).

**Таргет 2 — WS-ORIGIN-HARDCODED-LOCALHOST**
- `internal/transport/websocket/websocket.go`: `CheckOrigin` больше не хардкодит `{localhost:3000,5173,8080}` (в проде WS был сломан). Новый `OriginChecker(allowedOrigins)`:
  - отсутствие Origin разрешено (не-браузерные клиенты; браузеры всегда шлют Origin на WS-handshake);
  - пустой список → **same-origin-only** (Origin host == Host запроса, порт учитывается) — безопасный дефолт;
  - непустой список → exact-match, `"*"` или wildcard `"*.domain"` (симметрично CORS-политике).
- `cmd/api/main.go`: `STAIR_WS_ORIGINS` (приоритет) → fallback `STAIR_CORS_ORIGINS` → same-origin; `.env.example` документирует переменную.
- Тесты: новый `internal/transport/websocket/websocket_origin_test.go` (same-origin дефолт, порт-мисматч, malformed-Origin, exact, wildcard, `"*"`, интеграционный handshake allow/block); `TestOriginCheckAllowedAndBlocked` в `websocket_extra_test.go` переведён на явный конфиг-список.

## DoD-гейты (локально, закоммиченное состояние)

1. `gofmt -l .` — пусто
2. `go vet ./...` — 0
3. `go build ./...` + `go build ./cmd/...` — ok
4. `golangci-lint run` — **0 issues** (конфиг репо, v2.13.2)
5. `go test ./...` — **58/58 ok**, 0 FAIL
6. `go test -race -p 1 ./...` с postgres:16 (docker) — **58/58 ok** (0 FAIL, 0 SKIP), `internal/infrastructure/database` реально исполнен (4.152s) — БД-тесты не скипались, `STAIR_TEST_DATABASE_URL` задан; контейнер удалён после прогона

**Нюансы:**
- Первый БД-прогон упал на «Dirty database version 17» — это не регрессия кода, а переиспользованный контейнер-маскот прошлых волн (`stair-test-pg` держал порт 5432 с грязным состоянием миграций); известная проблема из отчёта W0 (`coverage-check.sh` без `-p 1`). Решение: контейнер пересоздан начисто → 58/58.
- Split на 2 коммита: W5-коммит содержит WS-auth-хендлер в промежуточном виде (3-arg конструктор без origin-пламбинга), W6-коммит добавляет пламбинг — я проверил, что **каждый коммит в отдельности компилируется** (`go build ./...` + тесты http/websocket пакетов).
- `REPORTS/` остаётся untracked по конвенции прошлых волн (отчёты в git не коммитятся; отчёт — файл `REPORTS/builder-S110-S112.md`).

**Рекомендации:**
- На проде: задать `STAIR_WS_ORIGINS` явно (`https://store.…,https://admin.…`); если не задан — унаследует `STAIR_CORS_ORIGINS` (в проде это strict-whitelist, не пусто, так что same-origin-дефолт НЕ активируется — это ожидаемо и безопасно).
- Проверить не-браузерных WS-клиентов (CLI/серверные): они не шлют Origin → пропускаются, поведение сохранено как раньше.
- S-113 (runtime-валидация 3 needs_validation) остаётся для деплой-волны.
