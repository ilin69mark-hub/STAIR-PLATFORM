# Review: PR #45 — security W1: S-104 + S-111 bootstrap guards

Branch: `dev/swarm/security-w1-s104-s111` → `main`
Commit: `06c2148` (+ `0fd349c` docs `.env.example`)

## Verdict: ✅ APPROVED

> self-approve forbidden → вердикт оформлен via comment + REPORTS, merge —
> по явной команде владельца (протокол репо).

## Чек-лист

| Проверка | Результат |
|---|---|
| `go build ./...` | ✅ clean |
| `go vet ./cmd/... ./internal/transport/...` | ✅ clean |
| `go test ./...` | ✅ все пакеты ok |
| Линт/стиль | ✅ названия, идиомы как в Stripe-guard ниже |
| Поведенческая регрессия (dev без флага / prod с флагом) | ✅ unit-тесты |
| Секреты в диффе | ✅ нет (`.env.production` gitignored, не ушёл) |

## Что проверено

### S-104 (api)
`cmd/api/main.go`: блок S1-2 расширен `else if isProduction → ERROR + os.Exit(1)`.
`isProduction` поднят выше (был в блоке Stripe-guard), дублирующий `env :=`/`isProduction :=`
внизу удалён. Stripe-guard ниже использует ту же переменную (без изменений логики).

### S-104 (worker)
`cmd/worker/main.go`: `if env != "production"` → `if !isProduction`
(EqualFold(production|prod)). **Ключевой фикс**: неканоничный
`STAIR_ENVIRONMENT` (например `prod`, `Production`) больше не включает
`AllowLoopback` в SSRF-политику webhook-доставки (WEBHOOK-MAPPED-LINKLOCAL-BYPASS).
Без этого фикса прод-при `prod` работал бы с loopback-доставкой.
Плюс fail-fast `else if isProduction` в блоке S1-2.

### S-111 (middleware)
`internal/transport/http/debug_log.go`:
- production → **безусловный** `return next` + warn (раньше флаг `STAIR_DEBUG_LOGGING=true`
  включал трейс тел и в проде);
- вне прода — включение только при флаге;
- `redactSensitive()` — маскирует значения `password|passwd|secret|token|authorization|cookie|api[-_]?key|client[-_]?secret`
  в JSON и form-encoded (заменяет `"value"`/`value` на `***`), применяется к req_body и resp_body.

### Тесты
`TestDebugLoggingMiddleware_RefusesProductionAlways` (production/prod/PRODUCTION + флаг=true → passthrough),
`_EnabledInDevOnly`, `_ProductionEmpty`, `TestRedactSensitive` (8 кейсов).
Нюанс: сравнение handler'ов — `debugStubHandler` (pointer struct), т.к. `http.HandlerFunc`
некомпарабельна в интерфейсе (panic при `!=`).

## Заметки

- Runtime fail-fast api не прогнан живьём: DB-ping (`database connect failed`) отрабатывает
  раньше блока ключей; логика идентична рабочему Stripe-guard — покрыто статикой.
- Helm: `secretsKey` → `STAIR_SECRETS_KEY` уже пробрасывается в `deployment.yaml`/`worker-deployment.yaml`;
  прод-деплой без ключа теперь fail-fast. Сделать helm-поле обязательным — **S-105 (W4)**.
- `.env.production` локальный и gitignored — правки документации там остались только локально;
  в PR ушёл только `.env.example`.