# STAIR PLATFORM

**Document:** EDR-0021_Advanced_Observability.md

**ID:** EDR-0021

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Scale

---

# 1. Purpose

Документ фиксирует продвинутую наблюдаемость (Phase H, H4) — эндпоинт
`/metrics` в Prometheus text-формате и расширенные HTTP-логи, реализованные
на чистой стандартной библиотеке Go (без prometheus client_golang), для
офлайн-сборки (DEV-0009). Скоуп H4:

1. **`GET /metrics`** — публичный эндпоинт в Prometheus text-формате с
   HTTP-метриками (request count/duration/size) и runtime-метриками
   (goroutines, memory, uptime).
2. **Реестр метрик** — собственный `internal/infrastructure/metrics`:
   Counter, Histogram, Gauge на `sync/atomic`, безопасные при конкуренции.
3. **Интеграция middleware** — сбор HTTP-метрик в `withLogging` (status уже
   доступен через `statusWriter`), расширение логов (`remote_ip`,
   `user_agent`).

Бизнес-метрики (`auth_login_total` и т.п.) — опциональное расширение
(не в первом релизе H4).

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase H — Scale, Advanced Observability)
- EDR-0018 (Horizontal Scaling — readiness, instance id)
- EDR-0019 (Regional Deployment — region label)
- API-0006 (Operational API: metrics endpoint)
- BE-0006 (Observability & Logging)
- ADR-0006 (Layered Architecture)

---

# 3. Model

## 3.1 Метрики (первый релиз)

| Имя | Тип | Labels | Описание |
|-----|-----|--------|----------|
| `http_requests_total` | Counter | method, path, status | Всего HTTP-запросов |
| `http_request_duration_seconds` | Histogram | method, path | Гистограмма длительности |
| `http_request_size_bytes` | Counter (sum/bytes) | method, path | Размер тел запросов |
| `go_goroutines` | Gauge | — | `runtime.NumGoroutine()` |
| `go_memstats_alloc_bytes` | Gauge | — | `runtime.ReadMemStats().Alloc` |
| `process_uptime_seconds` | Gauge | — | Время работы процесса |

Регион (EDR-0019) и instance id (EDR-0018) добавляются как label, если
заданы.

## 3.2 Реестр (stdlib)

`internal/infrastructure/metrics`:

```go
type Registry struct { /* counters/gauges/histograms */ }

func NewRegistry() *Registry
func (r *Registry) Counter(name, help string, labels ...string) *CounterVec
func (r *Registry) Gauge(name, help string) *Gauge
func (r *Registry) Histogram(name, help string, buckets []float64, labels ...string) *HistogramVec
func (r *Registry) Write(w io.Writer) error
```

- `CounterVec.With(labels...)` → атомарный счётчик.
- `HistogramVec.With(labels...).Observe(v)` — распределение по bucket'ам.
- `Gauge.Set(v)` — абсолютное значение.
- Подсчёт строго атомарный (sync/atomic): безопасно при конкурентных
  запросах нескольких реплик-горутин одного инстанса.

## 3.3 Prometheus text-формат

Writer выдаёт совместимый text-format (OpenMetrics-подмножество):

```
# HELP http_requests_total HTTP requests
# TYPE http_requests_total counter
http_requests_total{method="GET",path="/api/v1/projects",status="200"} 42
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{method="GET",path="/api/v1/projects",le="0.1"} 5
http_request_duration_seconds_bucket{... le="+Inf"} 10
http_request_duration_seconds_sum{...} 1.25
http_request_duration_seconds_count{...} 10
```

## 3.4 Интеграция

- `withLogging` собирает `http_requests_total{method,path,status}` и
  `http_request_duration_seconds`; записывает в реестр (статический
  `metricsRegistry`, инициализируемый в `applyConfig`).
- `/metrics` — публичный маршрут (вне auth/rate-limit): мониторинг не должен
  блокироваться собственным rate-limit'ом (инвариант).
- `remote_ip`, `user_agent` добавляются в лог-строки HTTP-запроса
  (наблюдаемость трафика).

---

# 4. Invariants

```
1. /metrics публичен и не подпадает под rate-limit/CSRF/auth.
2. МЕТРИКИ безопасны при конкуренции (sync/atomic); гонок нет.
3. text-формат совместим с Prometheus (нет дублей HELP/TYPE, корректное
   экранирование label-значений).
4. path в label ограничен каноническими сегментами (не сырой URL-query).
5. Бизнес-метрики — опционально; базовый набор покрыт (HTTP, runtime).
```

---

# 5. Schema

Без изменений схемы БД.

---

# 6. API

| Method | Path | Auth | Result |
|--------|------|------|--------|
| GET | `/metrics` | public | 200, `text/plain; version=0.0.4` |

---

# 7. Tests

- Metrics registry unit: counter incr, labels distinct, gauge set, histogram
  bucket assignment, text-format output (HELP/TYPE/samples/escaping).
- Concurrency: параллельные With()/Incr()/Observe() без гонок (race).
- Transport: `/metrics` публичен (нет auth/CSRF), счётчики увеличиваются
  после запросов, формат ответа `text/plain`.

---

# 8. Acceptance Criteria

- `GET /metrics` отдаёт HTTP + runtime метрики в Prometheus text-формате.
- Counters/histograms корректно инкрементятся под нагрузкой (нет гонок).
- `/metrics` доступен без аутентификации и не блокируется rate-limit'ом.
- Логи HTTP-запросов содержат remote_ip/user_agent.
- EDR-0021 помечен APPROVED; ROADMAP Phase H Advanced Observability CLOSED.

---

# 9. Changes

| Версия | Дата | Изменение |
|--------|------|-----------|
| 1.0.0 | 2026-08-15 | Первоначальная редакция (DRAFT) |
| 1.1.0 | 2026-08-15 | Реализовано (H4); APPROVED |

---

APPROVED