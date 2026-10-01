# builder-S144 — CSRF на 4 POST + Content-Type decodeJSON + generic 500 assistant (S-141 №7, №8)

**Задача:** [S-144] [P1] 4 мутирующих POST (`calculate`/`validate`/`optimize`/`assistant/{kind}`) висели на `authProtected` без CSRF; `decodeJSON` не требовал JSON Content-Type (вектор браузерной формы text/plain без preflight, CWE-352); assistant отдавал клиенту внутреннюю err-цепочку (CWE-209).

## Что сделано

1. **`internal/transport/http/router.go:142-147`** — `authProtected` → `authMutating` для:
   - `POST /api/v1/stairs:calculate`
   - `POST /api/v1/stairs:validate`
   - `POST /api/v1/stairs:optimize`
   - `POST /api/v1/assistant/{kind}` (при наличии assistantSvc)
   Теперь все четыре проходят `requireAuth` → `requireCSRF` (double-submit), как остальные mutating-роуты (S-108).
2. **`internal/transport/http/handler.go`** — `decodeJSON` для непустых тел требует `Content-Type: application/json` (префикс-match, регистронезависимо; параметры вроде charset пропускаются):
   - `hasBody` — `ContentLength > 0 || len(TransferEncoding) > 0` (chunked = непустое тело);
   - `contentTypeIsJSON` — media type до `;`, `EqualFold(trimSpace(...), "application/json")`.
   - Пустые тела/GET не затронуты; callers маппят ошибку в 400 `invalid_json`.
3. **`internal/transport/http/assistant.go`** — фиксированный текст 500 «Внутренняя ошибка. Попробуйте позже.» вместо `fmt.Sprintf("assistant: %v", err)`; детали err — только в `slog.Error`. Убран неиспользуемый импорт `fmt`.
4. **Тесты:**
   - `assistant_test.go` — новый хелпер `assistantAuthedRequest` (session+csrf cookie + `X-CSRF-Token` + `application/json`); все существующие тесты переведены на него.
     - **`TestAssistantInternalErrorGenericBody`** — ловушка №8: сбой модели с URL провайдера в err → 500, в теле нет `assistant:`/`openrouter`/`dial`, есть фиксированная фраза.
     - **`TestAssistantTextPlainNoCSRF`** — ловушка №7: POST text/plain с валидной session-cookie, БЕЗ CSRF-токена → **403 `code:"csrf"`** (requireCSRF раньше decodeJSON) — атака закрыта на уровне CSRF.
     - **`TestAssistantTextPlainValidCSRF`** — сценарий с валидным double-submit + text/plain: 400 `invalid_json` от decodeJSON, assistant НЕ вызван (`ast.kind == ""`), поведение зафиксировано.
     - **`TestMutatingStairRoutesRequireCSRF`** (handler_extra_test.go) — регресс-тест в стиле S-108: все 4 роута (calculate/validate/optimize/assistant) с session-cookie без CSRF-токена → **403 `code:"csrf"`** (табличный прогон на реальном роутере).
   - Рipple decodeJSON (явный `Content-Type: application/json` в тестах): `auth_test.go` (login/register/rate-limit), `handler_extra_test.go` (validatePublicRequest), `orders_test.go` (public consultations ×4 + invalid-JSON), `projects_test.go` (authedRequest), `projects_extra_test.go` (create-project handler-тесты), `public_test.go` (новый хелпер `publicQuoteRequest`, 13 сайтов).
   - Убран мёртвый хелпер `jsonRequest` из `testhelpers_test.go` (файл вернулся к исходному виду).

## Файлы

- `internal/transport/http/router.go` (+10/−8)
- `internal/transport/http/handler.go` (+21)
- `internal/transport/http/assistant.go` (+7/−6)
- тесты: `assistant_test.go` (+116/−31), `auth_test.go` (+10), `handler_extra_test.go` (+4/−2), `orders_test.go` (+5), `projects_extra_test.go` (+2), `projects_test.go` (+4/−2), `public_test.go` (+39/−19)

## Гейты

- `gofmt -l` — пусто
- `go vet ./...` — 0
- `go build ./...` + `go build ./cmd/...` — 0
- `go test -count=1 ./internal/transport/http/...` — ok
- `golangci-lint run` (пакет http, все изменённые файлы) — 0 issues
- Полный `STAIR_TEST_DATABASE_URL='postgres://stair:stair@localhost:5432/stair_test?sslmode=disable' go test -race -p 1 -count=1 -timeout 25m ./...` — **60/60 ok, exit 0**, повторён на финальном дереве (с `TestMutatingStairRoutesRequireCSRF`) — **60/60 ok** (двойной прогон в стиле S-137)

## Решения

- Порядок middleware `requireAuth` → `requireCSRF` даёт 401 без сессии и 403 `csrf` без токена — браузерная форма text/plain умирает на 403 ещё до `decodeJSON`.
- decodeJSON-гейт — второй рубеж (защита публичных/нес-аутентифицированных JSON-эндпоинтов и глубокой обороны): текст/тform-тело с валидным CSRF → 400, обработчик не вызывается.
- swagger.yaml не менялся: он уже декларирует `application/json` для этих эндпоинтов; поведение приведено к документированному.

## Блокеры

- Локальная БД `stair-test-pg` имеет пароль `stair` (не `testpassword` как в CI) — URL для локального полного прогона: `postgres://stair:stair@localhost:5432/stair_test?sslmode=disable`.
- Полный race-прогон на финальном дереве выполнен дважды подряд (движок variation ~6 мин каждый).

## Вердикт:

**S-144 ЗАКРЫТ.** Все 4 мутирующих POST под `authMutating`; text/plain-форма без CSRF → 403, с CSRF → 400 (обработчик не вызывается); err-цепочка assistant клиенту не раскрывается (фикс-текст 500). Тест-ловушки №7/№8 написаны и зелёные; полный race+БД прогон 60/60. Коммит ожидает пуша/PR (координатор).