# STAIR PLATFORM — Deployment (H1/H2, EDR-0018, EDR-0019)

Справочник по развёртыванию: горизонтальное масштабирование и региональные
схемы выкатки. Код: stateless API (EDR-0018), регион — наблюдаемостный тег
(EDR-0019).

## Топология

```
                    ┌────────────────────────────┐
   Edge/DNS ───────▶│ LB (реплики API)           │
                    │  - /health (liveness)      │
                    │  - /ready  (readiness)     │
                    └───────┬────────┬───────────┘
                            │        │
                     ┌──────▼──┐ ┌───▼──────┐
                     │  api-1  │ │  api-N   │  (STATAIR_REGION, STAIR_INSTANCE_ID)
                     └────┬────┘ └────┬─────┘
                          │           │
                     ┌────▼────────────▼─────┐
                     │  PostgreSQL (PRIMARY)  │──▶ read replicas (optional)
                     │  Redis (shared)        │
                     └────────────────────────┘
```

## Env-переменные, важные для развёртывания

| Env | Deфолт | Описание |
|-----|--------|----------|
| `STAIR_HTTP_ADDR` | `:8080` | Адрес HTTP-сервера |
| `STAIR_DATABASE_URL` | — | DSN PostgreSQL (обязателен) |
| `STAIR_REDIS_ADDR` | `""` | Адрес Redis (rate-limit, queue H3); пусто — memory fallback |
| `STAIR_INSTANCE_ID` | `""` | Идентификатор реплики (EDR-0018) |
| `STAIR_REGION` | `""` | Регион инстанса (EDR-0019) |
| `STAIR_SHUTDOWN_TIMEOUT` | `10s` | Таймаут graceful shutdown (drain) |

## Liveness / Readiness

- `GET /health` — liveness: всегда 200, не зависит от внешних систем.
- `GET /ready` — readiness: 200 только когда доступны БД (`SELECT 1`) и Redis
  (`PING`, если сконфигурирован); иначе 503. Настройте на это endpoint
  readinessProbe/LB, чтобы не выводить неготовые реплики в ротацию.

## Схемы выкатки

### Blue/green
Два полных окружения (current/next) + переключение трафика на edge/DNS.
`/ready` даёт «green» сняться из ротации, пока не готово; откат — мгновенное
возвращение на «blue».

### Canary
Постепенное расширение доли трафика на новые реплики через взвешенный LB,
наблюдая за `/metrics` (H4) и ошибками; откат — снижение веса.

## Требования

- Общий PostgreSQL (PRIMARY) и общий Redis для всех реплик одного региона.
- Реплики stateless: всё состояние пользователей в shared БД/Redis.
- Для нескольких регионов — один общий PRIMARY на инстанс данных (или
  репликация с мульти-региональной топологией) + регион как тег в метриках
  и логах.

## Примеры

- `deployments/docker-compose.yml` — локальный dev-стек (postgres+redis+api).
- `deployments/multi-region.example.yml` — multi-instance пример с region/instance
  метками и общими postgres/redis.
