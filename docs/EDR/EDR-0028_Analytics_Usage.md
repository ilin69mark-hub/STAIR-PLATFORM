# STAIR PLATFORM

**Document:** EDR-0028_Analytics_Usage.md

**ID:** EDR-0028

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Analytics

---

# 1. Purpose

Документ фиксирует Usage Analytics (Phase F, F1) — первый элемент
Analytics bounded context (BC-043). Цель — «кто и как использует
платформу»: агрегация активности tenant по временным бакетам (день /
неделя / месяц). Аналитика read-only: все данные уже персистентны в
операционных таблицах, новых миграций и write-path-хуков не вводим
(парсинг на лету, домен уже хранит полный снапшот в
`calculations.result`).

Состав:

1. **`internal/application/analytics`** — прикладной сервис аналитики
   (BC-043): entity + порт `Repository` + `Service`.
2. **`internal/infrastructure/database/analytics_repo.go`** — PostgreSQL
   агрегации: `UsageTotals` и `UsageSeries` (генерация непрерывного ряда
   бакетов через `generate_series` + `date_trunc` + `LEFT JOIN`).
3. **Транспорт** — `GET /api/v1/admin/analytics/usage` (auth+admin,
   право `analytics.read`).
4. **Frontend** — секция «Аналитика» в AdminPanel (Usage sub-view).

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase F — Analytics: Usage Analytics)
- EDR-0016 (Enterprise Controls — admin-эндпоинты и права)
- EDR-0013 (Audit Log — источник данных: auth.login, data.exported)
- ADR-0006 (Layered Architecture: application не зависит от БД/транспорта)
- ADR-0008 (Money, время, единицы)

---

# 3. Model

## 3.1 Метрики

| Метрика | Источник | Определение |
|---------|----------|-------------|
| `users` | `users` | Все учётные записи tenant |
| `active_users` | `audit_events` | Distinct `actor_id` событий `auth.login`/`result=ok` в окне |
| `projects` | `projects` | Все проекты tenant (в `totals`) |
| `projects_created` | `projects` | Проекты, созданные в окне (в `series`) |
| `calculations` | `calculations` ⋈ `projects` | Расчёты в окне (tenant-скоуп через проект) |
| `logins` | `audit_events` | `auth.login` + `result=ok` в окне |
| `exports` | `audit_events` | `data.exported` в окне |
| `payments` | `payment_intents` | Интенты `status=paid` в окне |

Гранулярность серий: `day` (дефолт), `week`, `month`. Ряд непрерывный —
пустые бакеты заполняются нулями (`generate_series`).

## 3.2 App-слой

```go
type Granularity string // "day" | "week" | "month"

type UsagePoint struct { // один бакет
    Bucket          time.Time
    Logins          int
    ActiveUsers     int
    ProjectsCreated int
    Calculations    int
    Exports         int
    Payments        int
}

type UsageTotals struct {
    Users        int
    ActiveUsers  int
    Projects     int
    Calculations int
    Logins       int
    Exports      int
    Payments     int
}

type UsageReport struct {
    From, To    time.Time
    Granularity Granularity
    Totals      UsageTotals
    Series      []UsagePoint
}

type Repository interface {
    UsageTotals(ctx, tenantID string, from, to time.Time) (UsageTotals, error)
    UsageSeries(ctx, tenantID string, from, to time.Time, g Granularity) ([]UsagePoint, error)
}

type Service struct{ repo Repository }
func (s *Service) Usage(ctx, tenantID, from, to, granularity) (*UsageReport, error)
```

Валидация: `from <= to`, гранулярность из каталога; ошибки
`ErrInvalidRange`, `ErrInvalidGranularity` (422 в транспорте).

## 3.3 SQL (репозиторий)

`UsageSeries` строит непрерывный ряд:

```sql
WITH buckets AS (
  SELECT generate_series(
    date_trunc($2, $3::timestamptz),
    date_trunc($2, $4::timestamptz),
    $5::interval
  ) AS bucket
)
SELECT b.bucket,
       COALESCE(login.n, 0), COALESCE(active.n, 0), COALESCE(proj.n, 0),
       COALESCE(calc.n, 0), COALESCE(export.n, 0), COALESCE(pay.n, 0)
FROM buckets b
LEFT JOIN (… logins …) login  ON login.bucket  = b.bucket
…
```

- `$2` — имя гранулярности (`day|week|month`), `$5` — шаг интервала
  (`1 day|1 week|1 month`).
- tenant-скоуп `calculations`: `JOIN projects p ON p.id = c.project_id
  AND p.tenant_id = $1` (SEC-0005).

`UsageTotals` — набор `COUNT(*)`/`COUNT(DISTINCT …)` подзапросов.

## 3.4 Транспорт

`GET /api/v1/admin/analytics/usage?from&to&granularity`

- `from`/`to` — `YYYY-MM-DD` или RFC3339; дефолт — окно 30 дней.
- `granularity` — `day|week|month`; дефолт — `day`.
- 200 — `UsageReport`; 403 — нет права `analytics.read`; 422 —
  невалидные параметры; 500 — внутренняя ошибка.

Право `analytics.read` добавлено в матрицу `RoleAdmin` (EDR-0015 §3.2 /
EDR-0016 §3.1) и доступно как scope API-ключа.

## 3.5 Frontend

Секция «Аналитика» в AdminPanel: карточки `totals` + таблица/серия по
бакетам. Вызов `analyticsApi.usage(from, to, granularity)`; новый файл
`frontend/src/api/analytics.ts`.

---

# 4. Security

- Только admin: проверка `analytics.read` (session-роль или scope
  API-ключа, единый путь `hasPermission`).
- Tenant-изоляция (SEC-0005): все запросы скоупированы по
  `tenantID` из аутентифицированного контекста.
- Параметры валидируются (422), чтение не мутирует данные.

---

# 5. Acceptance

1. `go build ./...`, `go vet ./...`, `gofmt -w` — чисто.
2. Unit-тесты сервиса (fake repo): валидация диапазона/гранулярности,
   проброс ошибок репозитория.
3. DB-тесты `analytics_repo_test`: totals и серия после
   пользователя/проекта/расчёта/аудита/платежа; пустые бакеты — нули.
4. Транспорт-тесты: 200 (admin), 403 (нет права), 422 (плохие
   параметры), 500 (ошибка сервиса).
5. ROADMAP: «Usage Analytics — DONE 2026-08-15».
