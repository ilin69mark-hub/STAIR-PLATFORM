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

## Frontend-сервисы

| Сервис | Порт | Назначение |
|--------|------|------------|
| store | 3000 | Клиентская витрина |
| admin | 5174 | Платформенная админка |
| store-admin | 5177 | Отдельная панель настроек текущего store |

Панель `store-admin` использует общую admin-сессию и backend-права `store.*`.


## Env-переменные, важные для развёртывания

| Env | Deфолт | Описание |
|-----|--------|----------|
| `STAIR_HTTP_ADDR` | `:8080` | Адрес HTTP-сервера |
| `STAIR_DATABASE_URL` | — | DSN PostgreSQL (обязателен) |
| `STAIR_REDIS_ADDR` | `""` | Адрес Redis (rate-limit, queue H3); пусто — memory fallback |
| `STAIR_VALIDATE_RATE_LIMIT` | `120` | Лимит публичного validate-эндпоинта (живая валидация при вводе, S-P5) с одного IP в минуту |
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

## Фоновые задания (Worker, EDR-0020/0035)

`cmd/worker` — отдельный процесс потребителя очереди (Redis List, fallback —
in-memory) и периодической очистки данных. В dev-стеке запускается сервисом
`worker` в том же `deployments/docker-compose.yml` (тот же образ, команда
`/bin/worker`):

- **Что делает**: ретраи/backoff до `MaxAttempts` (redis-очередь общая для
  всех реплик API+worker — EDR-0018/0020), выдаёт async-расчёты
  (`calc.calculate`, EDR-0035), доставку webhook/интеграций, очистку
  истёкших сессий/sso_states и аудит-ретенцию (`make up` ставит их сразу при
  старте — обработка идёт независимо от интервала).
- **Env**: `STAIR_CLEANUP_INTERVAL` (дефолт 1h), `STAIR_AUDIT_RETENTION_DAYS`
  (90), `STAIR_WORKER_SHUTDOWN_TIMEOUT` (10s), `STAIR_REDIS_ADDR`
  (=`redis:6379` в compose).
- **Трейсинг**: вместе с Jaeger (`make obs-tracing-up`) воркер экспортирует
  спаны как сервис `stair-platform-worker`; каждый обработанный job — span
  `job.process` (job.id/type/attempts, error при провале).

## Требования

- Общий PostgreSQL (PRIMARY) и общий Redis для всех реплик одного региона.
- Реплики stateless: всё состояние пользователей в shared БД/Redis.
- Для нескольких регионов — один общий PRIMARY на инстанс данных (или
  репликация с мульти-региональной топологией) + регион как тег в метриках
  и логах.

## Observability (EDR-0021, INFRA-0017/0019/0020/0021/0022)

Приложение отдаёт `/metrics` в Prometheus text-формате (кастомный реестр на
stdlib): HTTP (rate/latency/status, labels region/node, path канонизирован),
runtime (goroutines/mem/uptime), пул БД, circuit breaker, rate limiter,
response cache, compression и engine (`stair_calculate_duration_seconds`,
`stair_optimize_duration_seconds`). `/metrics` доступен только с внутренних
IP (`InternalOnlyMiddleware`), поэтому Prometheus скрейпит `api:8080` внутри
docker-сети проекта.

Стек поднимается оверрайдом поверх dev-стека:

```bash
make obs-up          # Prometheus :9090, Alertmanager :9093, Grafana :3030
make obs-tracing-up  # + Jaeger (OTLP :4318, UI :16686), STAIR_TRACING_ENABLED=true
make obs-down
make obs-config      # валидация merged compose
```

- Prometheus: `deployments/observability/prometheus/{prometheus,alerts}.yml`
  (13 alert rules: availability 99.9%, latency p95/p99, engine, БД-пул, CB,
  memory/goroutines, rate-limit).
- Alertmanager: `deployments/observability/alertmanager/alertmanager.yml`
  (receiver `default` — blackhole; добавьте webhook/email для доставки).
- Grafana: автопровижн datasource + дашборда `api-golden-signals`
  (deployments/observability/grafana).

Проверка конфигов:

```bash
docker run --rm -v "$PWD/deployments/observability/prometheus:/etc/prometheus" \
  --entrypoint promtool prom/prometheus:v2.55.0 check config /etc/prometheus/prometheus.yml
docker run --rm -v "$PWD/deployments/observability/alertmanager:/etc/alertmanager" \
  --entrypoint amtool prom/alertmanager:v0.27.0 check-config /etc/alertmanager/alertmanager.yml
```

## Примеры

- `deployments/docker-compose.yml` — локальный dev-стек (postgres+redis+api+store+admin+store-admin).
- `deployments/multi-region.example.yml` — multi-instance пример с region/instance
  метками и общими postgres/redis.
