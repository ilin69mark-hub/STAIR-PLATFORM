# builder-S143 — Sentry-скраб session/csrf-токенов (S-141 №2)

**Задача:** [S-143] [P0] Sentry-скраб: session/csrf-токены. Cookie `session`/`session_admin`/`csrf`/`csrf_admin` и заголовок `X-CSRF-Token` уходили в Sentry при каждой панике (аудит S-141 №2, HIGH, CWE-315/CWE-532).

## Что сделано

1. **`internal/infrastructure/redaction/redaction.go`** — в оба regex (JSON-поля + form/query) добавлены ключи:
   - `session` (в т.ч. `session_admin` через опциональный суффикс `(?:[-_]?admin)?`),
   - `csrf` / `csrf_admin`,
   - `stair_session` / `stair-session`.
   - Точное совпадение ключа: `session_id`/`session_name` не маскируются (регресс-кейс в тесте).
2. **`internal/infrastructure/sentry/scrub.go`** — `droppedHeaders` пополнен: `x-csrf-token`, `x-stair-signature` (double-submit CSRF и подпись запросов дропаются целиком, как Authorization/Cookie). Комментарии обновлены.
3. **Тесты:**
   - `redaction_test.go` — 8 новых кейсов (JSON/form, session/csrf, _admin-варианты, stair_session/stair-session, обратная совместимость session_id/session_name).
   - `sentry_test.go` — в `TestScrubEvent`: заголовки `X-CSRF-Token`/`X-Stair-Signature` дропаются; `Cookies: "session=SECRET; csrf=TKN"` (форма, которую кладёт `scope.SetRequest`) маскируется через redaction.
   - **Новый `TestCapturePanicScrubsSessionTokens`** — ловушка аудита: паника с валидными `session=SECRET` + `csrf=TKN` cookie и `X-CSRF-Token`/`X-Stair-Signature` заголовками → сериализованное событие после scrub-пайплайна не содержит ни SECRET, ни TKN, ни SIG.

## Файлы

- `internal/infrastructure/redaction/redaction.go` (+17/−14)
- `internal/infrastructure/redaction/redaction_test.go` (+9)
- `internal/infrastructure/sentry/scrub.go` (+13/−6)
- `internal/infrastructure/sentry/sentry_test.go` (+68/−13)

## Гейты

- `gofmt -l` — пусто
- `go vet ./...` — 0
- `go build ./...` + `go build ./cmd/...` — 0
- `go test ./internal/infrastructure/redaction/... ./internal/infrastructure/sentry/...` — ok
- `golangci-lint run` (3 пакета, все изменённые файлы в них) — 0 issues
- Полный `STAIR_TEST_DATABASE_URL='postgres://stair:stair@localhost:5432/stair_test?sslmode=disable' go test -race -p 1 -count=1 -timeout 25m ./...` — **60/60 ok** (до финальных правок тестов; финальный прогон после обеих задач — в S-144)

## Решения

- Маска `***` для значений, дроп для заголовков — заодно с существующим паттерном S-140 (хедеры неизвестной схемы регулярка не поймает).
- `session_id`/`session_name` сознательно не маскируются (не токены; `session`-альтернация не матчится на них благодаря точному совпадению ключа).

## Блокеры

Нет.

## Вердикт:

**S-143 ЗАКРЫТ.** Код+Sentry-payload не содержат session/csrf-токенов; тест-ловушка аудита (паника → scrub → нет SECRET/TKN/SIG) написана и зелёная; полный race+БД прогон зелёный. Коммит ожидает пуша/PR (координатор).