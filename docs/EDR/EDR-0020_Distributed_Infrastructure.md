# STAIR PLATFORM

**Document:** EDR-0020_Distributed_Infrastructure.md

**ID:** EDR-0020

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Scale

---

# 1. Purpose

Документ фиксирует распределённую инфраструктуру (Phase H, H3) — систему
фоновых заданий (job queue) для распределённой/асинхронной обработки и
регулярной очистки данных. Скоуп H3:

1. **Job queue** — интерфейс отложенных заданий с двумя бэкендами:
   распределённый (Redis List) и in-memory fallback (single-instance/тесты).
2. **Worker** — отдельный процесс `cmd/worker`, потребитель очереди
   (BRPOP, retry с backoff, max-attempts, graceful shutdown).
3. **Очистка данных** — первый реальный класс заданий: удаление истёкших
   сессий, истёкших sso_states, устаревших аудит-событий (retention).
   Сейчас такие строки в БД не удаляются — накапливаются бесконечно.

Реализация — офлайн-безопасна: stdlib (net, encoding/json, crypto/rand,
sync) + go-redis (уже в проекте, v9) + pgxpool (уже в проекте). Без новых
внешних зависимостей.

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase H — Scale, Distributed Infrastructure)
- EDR-0018 (Horizontal Scaling — stateless, readiness)
- EDR-0013 (Audit Log)
- EDR-0014 (Advanced Security)
- EDR-0017 (SSO)
- SEC-0002 (Session Management)
- API-0007 (Background Jobs / Worker)

---

# 3. Model

## 3.1 Job

```go
type Job struct {
    ID          string            `json:"id"`
    Type        string            `json:"type"`
    Payload     json.RawMessage   `json:"payload"`
    Attempts    int               `json:"attempts"`
    MaxAttempts int               `json:"max_attempts"`
    CreatedAt   time.Time         `json:"created_at"`
}
```

`MaxAttempts` — предел попыток (по умолчанию 3). При превышении задание
считается «битым» и логируется (не пере-обрабатывается) — избыточная
защита от бесконечного ретрая.

## 3.2 JobQueue (интерфейс)

```go
type JobQueue interface {
    Enqueue(ctx context.Context, job Job) error
    Dequeue(ctx context.Context) (Job, bool, error) // false — пусто
}
```

Бэкенды:

- **Redis List** (`redisQueue`): распределённая очередь на FIFO-списке Redis
  (`LPUSH` enqueue, `BRPOP` dequeue). Все реплики/воркеры видят общую
  очередь; распределённость достигается фактом общего Redis.
- **In-memory** (`memoryQueue`): FIFO-буфер с мьютексом/каналом внутри
  процесса. Используется как fallback при недоступности Redis и в тестах
  (single-instance). Паттерн соответствует rate-limiter (EDR-0014 §3.2).

## 3.3 Типы заданий

| Type | Payload | Назначение |
|------|---------|------------|
| `sys.cleanup_sessions` | `{"before":"RCF3339"}` | Удалить истёкшие сессии (`expires_at < before`) |
| `sys.cleanup_sso_states` | `{}` | Удалить истёкшие sso_states (`expires_at < now()`) |
| `sys.cleanup_audit` | `{"before":"RCF3339","retention_days":90}` | Удалить аудит-события старше retention |

Расширяемость: новый `Type` добавляется в registry воркера (switch), не
меняя ядро очереди.

## 3.4 Воркер (`cmd/worker`)

Отдельный процесс (запускается независимо от API):

```
1. Подключение к БД (pgxpool) и Redis (если задан).
2. Выбор бэкенда очереди: Redis при доступности, иначе in-memory (fallback).
3. Бесконечный цикл потребителя:
   a. Dequeue (BRPOP, блокирующий таймаут ~2s);
   b. Реестр: по job.Type выбрать обработчик;
   c. Обработка; при ошибке — retry с backoff, инкремент Attempts, до
      MaxAttempts; исчерпание — лог «job failed permanently».
4. Периодический таймер (каждые ~1ч, настраивается): enqueue заданий
   очистки (sessions/sso_states/audit).
5. Graceful shutdown по SIGTERM/SIGINT: drain текущего задания.
```

Конфигурация env:

| Env | Deфолт | Описание |
|-----|--------|----------|
| `STAIR_DATABASE_URL` | — | DSN PostgreSQL (обязателен) |
| `STAIR_REDIS_ADDR` | `""` | Redis; пусто — in-memory queue |
| `STAIR_CLEANUP_INTERVAL` | `1h` | Период enqueue заданий очистки |
| `STAIR_AUDIT_RETENTION_DAYS` | `90` | Retention аудита |
| `STAIR_WORKER_SHUTDOWN_TIMEOUT` | `10s` | Drain |

## 3.5 Очистка данных (repo-методы)

Новые методы (инфраструктурные, в `internal/infrastructure/database`):

- `auth_repo.go`:
  - `DeleteExpiredSessions(ctx, before) (int64, error)` —
    `DELETE FROM sessions WHERE expires_at < $1`;
  - `DeleteExpiredSsoStates(ctx) (int64, error)` —
    `DELETE FROM sso_states WHERE expires_at < now()`.
- `audit_repo.go`:
  - `DeleteBefore(ctx, before) (int64, error)` —
    `DELETE FROM audit_events WHERE created_at < $1`.

Возвращают число удалённых строк (для логгирования/метрик).

---

# 4. Invariants

```
1. Очередь FIFO; Dequeue не теряет задания (возвращает ok=false при пусто).
2. Redis-бэкенд используется при доступности Redis; иначе in-memory
   (fallback, паттерн EDR-0014).
3. Задание с ошибкой ретраится с backoff до MaxAttempts; превышение —
   окончательный отказ (лог), не бесконечный цикл.
4. Очистка всегда безопасна: только истёкшие/устаревшие строки; никогда не
   трогает активные сессии/sso_states/audit в пределах retention.
5. Воркер завершается graceful (дренаж текущего задания) по SIGTERM/SIGINT.
6. Очистка не блокирует API: выполняется в отдельном процессе (cmd/worker).
```

---

# 5. Schema

Без изменения схемы (очередь — Redis-список, не таблица). Очистка использует
существующие таблицы `sessions`, `sso_states`, `audit_events`. Индексы уже
покрывают запросы (audit: (tenant_id, created_at)).

---

# 6. API / Команды

Изменений HTTP API нет. Новая команда запуска:

```
STAIR_DATABASE_URL="..." STAIR_REDIS_ADDR="..." go run ./cmd/worker
```

---

# 7. Tests

- **Queue (in-memory)**: enqueue/dequeue FIFO, dequeue на пустой → ok=false,
  порядок сохранён, конкурентный enqueue/dequeue (race).
- **Queue (redis, optional)**: LPUSH/BRPOP round-trip на реальном Redis
  (интеграционный, при доступности).
- **Cleanup repo (БД)**: DeleteExpiredSessions удаляет только истёкшие;
  DeleteExpiredSsoStates — только истёкшие; DeleteBefore — старше границы.
- **Worker**: registry dispatch по типу; retry/backoff/max-attempts;
  обработчик очистки с mock/реальной БД.

---

# 8. Acceptance Criteria

- `JobQueue` работает на Redis (распределённо) и на in-memory (fallback).
- `cmd/worker` потребляет задания, ретраит ошибки, graceful shutdown.
- Очистка удаляет истёкшие сессии, sso_states и устаревшие аудит-события;
  активные данные не затрагиваются.
- Таймер воркера периодически ставит задания очистки.
- EDR-0020 помечен APPROVED; ROADMAP Phase H Distributed Infrastructure
  CLOSED.

---

# 9. Changes

| Версия | Дата | Изменение |
|--------|------|-----------|
| 1.0.0 | 2026-08-15 | Первоначальная редакция (DRAFT) |
| 1.1.0 | 2026-08-15 | Реализовано (H3); APPROVED |

---

APPROVED
