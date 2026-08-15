# STAIR PLATFORM

**Document:** EDR-0035_Distributed_Processing.md

**ID:** EDR-0035

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Engineering

---

# 1. Purpose

Документ фиксирует Distributed Processing (Phase B, B4) — финальный
элемент Engineering Expansion bounded context (BC-002). Цель — разгрузить
API-процесс от длительных расчётов: постановка расчёта в фоновую очередь
(EDR-0020), выполнение в воркере (`cmd/worker`), хранение статуса и
результата в БД, опрос клиентом по `job_id`.

Модель: **async-расчёт через JobQueue**. API только валидирует вход,
создаёт запись `calc_jobs` со статусом `pending` и ставит задание
`calc.calculate` в очередь; воркер выполняет полный конвейер
(`stair.Service.Calculate`) и фиксирует успех (JSON результата) или
ошибку. Повторные попытки — штатная политика воркера (retry/backoff,
EDR-0020 инвариант 3); каждая попытка пересчитывает с нуля (расчёт и
так детерминирован — ADR-0003).

Асинхронный расчёт полезен для: тяжёлых конфигураций в производственных
интеграциях (API-ключ, EDR-0016), когда клиенту не нужен мгновенный
ответ; будущего распределённого планирования по регионам (EDR-0019);
батч-пересчётов.

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase B — Engineering Expansion: Distributed Processing)
- ADR-0003 (детерминизм: повторный расчёт воркером эквивалентен
  исходному)
- EDR-0020 §3 (JobQueue: Redis List + in-memory fallback, retry/backoff,
  max-attempts — переиспользуется как-есть)
- EDR-0018 (Horizontal Scaling: воркер и API — отдельные процессы,
  очередь на Redis — общая точка координации)
- `internal/application/jobs` (прикладной сервис фоновых заданий),
  `internal/infrastructure/queue` (очередь), `cmd/worker` (потребитель),
  `internal/infrastructure/database/calc_job_repo.go` (хранилище статуса)

---

# 3. Model

## 3.1 Job type `calc.calculate`

Новый тип задания в JobQueue (`queue.JobCalcCalculate`). Payload задания
максимально лёгкий — только координаты записи в БД:

```json
{ "job_id": "<calc_jobs.id>", "tenant_id": "<uuid>" }
```

Вся расчётная информация хранится в таблице `calc_jobs`; воркер читает
её по `job_id`. Это делает ретраи простыми: повторная попытка загружает
тот же payload и пересчитывает.

## 3.2 Table `calc_jobs` (migration 000016)

| column       | type          | notes                                        |
|--------------|---------------|----------------------------------------------|
| `id`         | TEXT PK       | id задания из очереди (32 hex)               |
| `tenant_id`  | UUID NOT NULL | FK `tenants`, cascade delete                 |
| `user_id`    | UUID NULL     | FK `users`, set null                         |
| `type`       | TEXT NOT NULL | `calc.calculate`                            |
| `status`     | TEXT NOT NULL | `pending`/`running`/`succeeded`/`failed`     |
| `payload`    | JSONB NOT NULL | `{config, options}` — входные данные расчёта |
| `result`     | JSONB NULL    | сериализованный `stair.Result` при успехе    |
| `error`      | TEXT NULL     | текст ошибки при провале                     |
| `created_at` | TIMESTAMPTZ   | when enqueued                                |
| `started_at` | TIMESTAMPTZ   | when worker взял в работу                     |
| `finished_at`| TIMESTAMPTZ   | when succeeded/failed                        |

Индексы: `(tenant_id)`, `(status)`.

## 3.3 Прикладной сервис `application/jobs`

- `Service.SubmitCalculate(ctx, tenantID, userID, payload)` — генерирует
  `job_id`, создаёт запись `pending`, ставит `calc.calculate` в очередь.
  Порядок: сначала запись в БД, затем enqueue; сбой enqueue помечает запись
  `failed` и возвращает ошибку (запись «не сирота»).
- `Service.GetJob(ctx, tenantID, id)` — статус+результат (скоуп tenant'ом).
- `Service.RunCalculate(ctx, tenantID, id)` — вызывается воркером:
  `running` → вычисление (инъектированная функция расчёта) →
  `succeeded`(result JSONB) / `failed`(error, контекст отмены — тоже
  failure). Расчёт детерминирован, поэтому повторные попытки консистентны.

Порт `Repository` (Create/GetByID/MarkRunning/MarkSucceeded/MarkFailed)
реализован `database.CalculationJobRepository`; в юнит-тестах — фейк.

## 3.4 Transport

- `POST /api/v1/stairs:calculate/async` (auth+CSRF) — тело то же, что у
  `POST /api/v1/stairs:calculate`; вход валидируется синхронно
  (невалидный вход → 422 сразу, в очередь не попадает). Ответ `202`:
  `{job_id, type, status: "pending"}`.
- `GET /api/v1/jobs/{id}` (auth) — `{id, type, status, result?, error?}`;
  `result` — в том же DTO-формате, что синхронный расчёт. 404 — нет записи
  у данного tenant'а.

---

# 4. Invariants

1. **Не сирота**: каждая запись `calc_jobs` имеет либо enqueue-успех
   (`pending`), либо явный `failed`; очередь не остаётся бесследно без
   записи.
2. **Результат консистентен**: `succeeded` ⇒ `result` непустой валидный
   JSON; `failed` ⇒ `error` непустой.
3. **Скоуп tenant'а**: `GetJob` недоступен из другого tenant'а (404).
4. **Детерминизм**: повторный запуск воркером даёт тот же результат
   (ADR-0003); retry не меняет выходные данные, только `attempts`.
5. **Очередь не знает о расчётах**: payload задания не содержит
   конфигурации; все данные — в `calc_jobs`.

---

# 5. Non-goals

- Распределённое выполнение одного расчёта между воркерами (декомпозиция
  конвейера) — вне scope; конвейер выполняется в одном задании.
- Приоритеты/дедлайны заданий, отмена после постановки, recovery записей
  «застрявших в running» после падения воркера (поток сознательно
  упрощён; финальная отдача после успешного retry перекрывает текущий
  процесс).
- Async для project-calculation (`projects/{id}/calculate/async`) и
  optimize — потенциальное расширение (тот же механизм, другой тип).

---

# 6. Decision

Реализовано в Phase B, B4 (commit `feat(b4)`): новый пакет
`internal/application/jobs`, таблица `calc_jobs` (000016), типа задания
`calc.calculate`, эндпоинты `POST /api/v1/stairs:calculate/async` → 202 и
`GET /api/v1/jobs/{id}`, обработчик в реестре воркера. Очередь и политика
ретраев переиспользуют EDR-0020 без изменений.