# builder — S-119: WS role-гейт на /ws/admin

**Задача:** S-119 [P2] — `/ws/admin` принимал любую валидную `session_admin`
cookie без проверки роли (cookie получает любой зарегистрированный юзер).
Перед активацией realtime обязателен role-гейт.

**Выполнено:** координатором (self-execute).

## Изменения

- `internal/transport/http/websocket_handler.go`:
  - `TokenValidator.Authenticate` теперь возвращает `(userID, role, err)` —
    роль пробрасывается из auth-сервиса без лишних лукапов.
  - После аутентификации: admin-namespace (`/ws/admin`) + роль ≠ `admin` →
    **403** (authenticated, но не authorized). Store-namespace (`/ws`) открыт
    всем аутентифицированным. Гейт по пути, не по методу auth (cookie и Bearer
    одинаково).
- `cmd/api/main.go` — `authTokenValidator` возвращает `user.ID + string(user.Role)`.
- `internal/transport/http/websocket_handler_test.go`:
  - мок: поле `role` (дефолт `"user"`), `validateFunc` с тройным возвратом.
  - `TestWebSocketHandlerWithAdminSessionCookie` — теперь с ролью admin.
  - НОВЫЕ: `TestWebSocketHandlerNonAdminRejectedOnAdminPath` (user + admin-cookie
    на /ws/admin → 403) и `TestWebSocketHandlerBearerRoleGateOnAdminPath`
    (user Bearer → 403, admin Bearer → 200/400).

## Гейты (локально)

| Гейт | Результат |
|---|---|
| gofmt -l | пусто |
| go vet ./... | 0 |
| go build ./... | ok |
| golangci-lint run | 0 issues |
| WS-тесты | 17/17 PASS (включая 3 новых кейса) |
| **Полный прогон** `go test -race -p 1 ./...` + БД | **58/58 ok, 0 FAIL** |

**Вердикт:** S-119 закрыт — /ws/admin доступен только роли admin (403 для
остальных при валидной аутентификации); /ws без изменений. 58/58, lint 0.

**Длительность:** ~25 мин (self-execute).