# builder-S132c — Авторизация WS-подписок на pipeline-комнаты

**Вердикт:** S-132c DONE — подписка на `pipeline:<configID>` теперь требует
членства в проекте конфигурации; admin — bypass. Полный прогон
`go test -race -p 1` с БД: **58 ok, 0 FAIL**; golangci-lint: 0 issues.

## Что сделано

Закрыт capability-гэп: раньше любой аутентифицированный клиент мог
подписаться на `pipeline:<uuid>` и получать статусы расчётов чужого проекта
(UUID = capability). Теперь подписка проверяется по членству.

### Слои (DDD, порт-адаптер)

1. **application/project** — новый use-case `HasConfigAccess(ctx, userID, configurationID)`:
   - `repository.go`: метод добавлен в интерфейс `Repository`.
   - `service.go`: `HasConfigAccess` — пустые userID/configID → false (без
     доменной проверки); делегирует в repo.
   - `project_repo.go` (infrastructure): SQL
     `SELECT EXISTS (SELECT 1 FROM stair_configurations sc JOIN project_members pm ON pm.project_id = sc.project_id WHERE sc.id = $1 AND pm.user_id = $2)`
     — не-член/нет конфига → false без ошибки.

2. **transport/websocket** — порт `SubscriberAuthorizer`:
   - `CanSubscribe(userID, role, room string) bool`.
   - `Client` получил поля `role` + `authorizer`; опции `WithRole`,
     `WithSubscriberAuthorizer`; `HandleWebSocket` принимает `opts ...ConnectOption`
     (обратная совместимость — старые вызовы не ломаются).
   - `handleSubscription`: при `MessageTypeSubscribe` и заданном авторизаторе
     отказ → денай без ack (клиент остаётся вне комнаты). Nil-авторизатор —
     legacy-поведение (подписка без проверки), прод всегда задаёт проверку.

3. **transport/http** — `WebSocketHandler.SetSubscriberAuthorizer(a)` +
   проброс `role`/`authorizer` в `ws.HandleWebSocket`.

4. **cmd/api** — `pipelineSubscriberAuthorizer` (admin → allow без surplus-
   проверки; иначе `HasConfigAccess`); подключён к WS-обработчику.

## Тесты

- `internal/transport/websocket/subscribe_auth_test.go` — allow/deny/role
  проброс (3 теста).
- `internal/application/project/service_test.go` — `TestHasConfigAccess`
  (owner/stranger/пустые ID/после AddMember).
- `internal/infrastructure/database/project_repo_test.go` —
  `TestProjectRepositoryHasConfigAccess` (интеграционный, реальный PG).
- `cmd/api/main_test.go` — `TestPipelineSubscriberAuthorizerAdminBypass`,
  `TestPipelineSubscriberAuthorizerMember` (admin bypass без БД).
- Обновлены моки: `fakeRepo`, `MockProjectRepository` (graphql).

## Гейты

- `go build ./...` — ok
- `go vet ./internal/... ./cmd/...` — ok
- `golangci-lint run` (затронутые пакеты) — 0 issues
- `go test -race -p 1 -count=1 -timeout 25m ./...` с
  `STAIR_TEST_DATABASE_URL` (postgres:16, docker) — **58 ok, 0 FAIL**
  (БД сброшена перед прогоном: dirty version 10 от предыдущих прогонов).

## Файлы

- `internal/application/project/repository.go` — интерфейс + `HasConfigAccess`
- `internal/application/project/service.go` — use-case
- `internal/infrastructure/database/project_repo.go` — SQL-реализация
- `internal/transport/websocket/websocket.go` — порт + опции + гейт
- `internal/transport/http/websocket_handler.go` — SetSubscriberAuthorizer
- `cmd/api/main.go` — pipelineSubscriberAuthorizer + wiring
- тесты: `subscribe_auth_test.go`, `service_test.go`, `project_repo_test.go`,
  `main_test.go`, `mock_repo_test.go`

## Решения

- Nil-авторизатор = legacy (без проверки), прод всегда задаёт проверку —
  обратная совместимость для тестов/старых вызовов.
- Admin bypass живёт в http-реализации (знает про роли), ws-слой остаётся
  транспортно-агностичным.
- Отказ при не-членстве — без ack (клиент просто не получает статусы),
  лог на сервере.

## Блокеры

Нет.