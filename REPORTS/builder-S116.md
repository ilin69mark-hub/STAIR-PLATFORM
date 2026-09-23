# builder — S-116: WS admin-namespace по пути /ws/admin

**Задача:** S-116 [P3] (SHOULD из ревью PR #50) — `sessionToken()` выбирал
`session_admin` по заголовку `X-App-Origin`, но браузерный WebSocket API не
позволяет ставить кастомные заголовки → admin-хендшейк брал store-cookie и
получал 401. Тест `WithAdminSessionCookie` сам подставлял заголовок, маскируя
проблему.

**Выполнено:** координатором (self-execute; Task-dispatch в сессии
нестабилен — fallback по протоколу §9.1).

## Изменения

- `internal/transport/http/websocket_handler.go`:
  - `wsAppOrigin(r)` — namespace по пути: `/ws/admin` → admin, иначе store.
    Заголовок `X-App-Origin` на WS намеренно НЕ используется (браузер его не
    пошлёт; не-браузерные клиенты идут через Bearer).
  - `wsSessionToken(r)` — session-cookie для namespace из пути.
  - `HandleWebSocket` использует `wsSessionToken` вместо `sessionToken`.
- `internal/transport/http/router.go` — регистрация `GET /ws/admin` рядом с
  `GET /ws` (комментарий про S-116).
- `internal/transport/http/websocket_handler_test.go`:
  - `TestWebSocketHandlerWithAdminSessionCookie` — переведён на путь
    `/ws/admin` (реальный браузерный сценарий), заголовок убран.
  - НОВЫЕ: `TestWebSocketHandlerAdminCookieRejectedOnStorePath` (admin-cookie
    на `/ws` → 401) и `TestWebSocketHandlerStoreCookieRejectedOnAdminPath`
    (store-cookie на `/ws/admin` → 401) — namespace разделён жёстко.
- `internal/transport/http/swagger_sync_test.go` — `/ws/admin` в
  nonRESTRoutes.

## Гейты (локально)

| Гейт | Результат |
|---|---|
| gofmt -l | пусто |
| go vet ./... | 0 |
| go build ./... | ok |
| golangci-lint run | 0 issues |
| go test -count=1 ./internal/transport/http/... | ok (4.99s) |
| WS-тесты (15, включая 2 новых негативных) | 15/15 PASS |
| **Полный прогон** `go test -race -p 1 ./...` + БД (postgres:16) | **58/58 ok, 0 FAIL** |

## Решения

- Выбран path-based вариант (из двух предложенных в карточке: путь `/ws/admin`
  или документировать Bearer-only). Путь — единственный надёжный способ для
  браузера различить namespace; Bearer-only оставил бы admin без cookie-пути
  вовсе.
- Фронтенды WS пока не используют (grep = 0) — ломать нечего; при подключении
  realtime admin-фронт должен ходить на `/ws/admin`, store — на `/ws`.

**Вердикт:** S-116 реализован — admin-WS аутентифицируется по пути
/ws/admin (cookie session_admin), namespace разделён и покрыт негативными
тестами; 58/58 локально (race+БД), lint 0.

**Длительность:** ~20 мин (self-execute).

**Нюансы:** Task-dispatch отменяется систематически (3/3 в сессии) —
координатор работает self-execute.

**Рекомендации:** при подключении realtime на фронтах: admin → `/ws/admin`,
store → `/ws`; счётчик задач = 2/20, CI не требуется.