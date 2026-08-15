# STAIR PLATFORM

**Document:** EDR-0029_Analytics_Projects.md

**ID:** EDR-0029

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Analytics

---

# 1. Purpose

Документ фиксирует Project Analytics (Phase F, F2) — второй элемент
Analytics bounded context (BC-043). Цель — «состояние и активность
проектов tenant»: сводка по каждому проекту (число конфигураций,
расчётов, комментариев, участников, последний расчёт и его валидность)
и агрегаты tenant (распределение по статусам, проекты с валидным
расчётом, итоговые счётчики). Как и Usage Analytics (EDR-0028),
аналитика read-only: данные персистентны в операционных таблицах,
новых миграций и write-path-хуков нет.

Состав:

1. **App-слой** — расширение `internal/application/analytics`:
   типы `ProjectRow`, `ProjectTotals`, `ProjectReport`, метод
   `Service.Projects`.
2. **Репозиторий** — `internal/infrastructure/database/analytics_repo.go`:
   `ProjectTotals` и `ProjectList` (tenant-скоуп через `projects.tenant_id`,
   подзапросы по конфигурациям/расчётам/комментариям/участникам).
3. **Транспорт** — `GET /api/v1/admin/analytics/projects` (auth+admin,
   право `analytics.read`).
4. **Frontend** — секция «Проекты» в AdminPanel (таблица сводок по
   проектам + агрегаты).

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase F — Analytics: Project Analytics)
- EDR-0028 (Usage Analytics — общая трубка: право, валидация, DTO)
- EDR-0008 (Project membership), EDR-0009 (Comments), EDR-0010 (Review),
  EDR-0012 (Configuration versioning) — источники данных
- ADR-0006 (Layered Architecture), ADR-0008 (Money, время, единицы)

---

# 3. Model

## 3.1 Метрики

Агрегаты tenant (окно `[from, to]`):

| Метрика | Источник | Определение |
|---------|----------|-------------|
| `projects` | `projects` | Все проекты tenant |
| `projects_created` | `projects` | Созданы в окне |
| `by_status` | `projects` | Распределение по `status` (draft/in_review/approved/changes_requested) |
| `projects_with_calculation` | `calculations ⋈ projects` | Distinct проектов с ≥1 расчётом |
| `valid_projects` | `calculations ⋈ projects` | Проекты, чей последний расчёт `valid` |
| `configurations` | `stair_configurations ⋈ projects` | Все конфигурации tenant |
| `calculations` | `calculations ⋈ projects` | Все расчёты tenant |
| `comments` | `project_comments ⋈ projects` | Все комментарии tenant |

Сводка по проекту (на момент `to`):

| Поле | Источник |
|------|----------|
| `id`, `name`, `status`, `created_at`, `updated_at` | `projects` |
| `owner_email` | `users` (LEFT JOIN по `owner_id`) |
| `configurations` | `COUNT(stair_configurations)` |
| `calculations` | `COUNT(calculations)` |
| `latest_calculation_valid` | `valid` последнего расчёта (nullable) |
| `comments` | `COUNT(project_comments)` |
| `members` | `COUNT(project_members)` |

## 3.2 App-слой

```go
type ProjectRow struct {
    ID, Name, Status, OwnerEmail   string
    CreatedAt, UpdatedAt           time.Time
    ConfigurationCount             int
    CalculationCount               int
    LatestCalculationValid         *bool // nil — нет расчётов
    CommentCount, MemberCount      int
}

type ProjectTotals struct {
    Projects, ProjectsCreated int
    ByStatus                  map[string]int
    ProjectsWithCalculation   int
    ValidProjects             int
    Configurations            int
    Calculations              int
    Comments                  int
}

type ProjectReport struct {
    From, To time.Time
    Totals   ProjectTotals
    Projects []ProjectRow
}
```

Порт `Repository` расширяется двумя методами:

```go
ProjectTotals(ctx, tenantID string, from, to time.Time) (ProjectTotals, error)
ProjectList(ctx, tenantID string) ([]ProjectRow, error)
```

Сервис:

```go
func (s *Service) Projects(ctx, tenantID string, from, to time.Time) (*ProjectReport, error)
```

Валидация: `from <= to` (тот же `ErrInvalidRange` → 422).

## 3.3 SQL (репозиторий)

`ProjectTotals` — набор подзапросов, tenant-скоуп через `projects.tenant_id`:

```sql
SELECT COUNT(*) FROM projects WHERE tenant_id = $1;                                   -- projects
SELECT COUNT(*) FROM projects WHERE tenant_id = $1 AND created_at BETWEEN $2 AND $3;  -- created
SELECT status, COUNT(*) FROM projects WHERE tenant_id = $1 GROUP BY status;           -- by_status
-- valid_projects: DISTINCT ON последний расчёт каждого проекта
SELECT COUNT(*) FROM (
  SELECT DISTINCT ON (c.project_id) c.project_id, c.valid
  FROM calculations c JOIN projects p ON p.id = c.project_id AND p.tenant_id = $1
  ORDER BY c.project_id, c.created_at DESC
) latest WHERE latest.valid;
```

`ProjectList` — по одной строке на проект, агрегаты подзапросами (или
`JOIN` + `GROUP BY`), сортировка по `updated_at DESC`.

## 3.4 Транспорт

`GET /api/v1/admin/analytics/projects?from&to`

- `from`/`to` — `YYYY-MM-DD` или RFC3339; дефолт — окно 30 дней
  (одинаковая трубка `queryTime`, что в EDR-0028).
- 200 — `ProjectReport`; 403 — нет права `analytics.read`; 422 —
  невалидный диапазон; 500 — внутренняя ошибка.

DTO: `projects_created`, `by_status`, `projects_with_calculation`,
`valid_projects`, `configurations`, `calculations`, `comments`; массив
`projects` с полями из §3.1.

## 3.5 Frontend

Секция «Аналитика использования» в AdminPanel дополняется подсекцией
«Проекты»: агрегаты (карточки) + таблица проектов со сводкой
(статус, конфигурации, расчёты, последний расчёт валиден, комментарии,
участники). Вызов `analyticsApi.projects(from, to)` в
`frontend/src/api/analytics.ts`; типы — в `types.ts`.

---

# 4. Security

- Только admin: право `analytics.read` (session-роль или scope API-ключа,
  единый путь `hasPermission`).
- Tenant-изоляция (SEC-0005): все запросы скоупены по `tenantID` из
  аутентифицированного контекста; `owner_email` раскрывается только
  внутри tenant.
- Параметры валидируются (422), чтение не мутирует данные.

---

# 5. Acceptance

1. `go build ./...`, `go vet ./...`, `gofmt -w` — чисто.
2. Unit-тесты сервиса: валидация диапазона, проброс ошибок репозитория,
   формирование отчёта из fake-данных.
3. DB-тесты `analytics_repo_test`: totals и список после создания
   проектов/конфигураций/расчётов/комментариев/членов; изоляция tenant.
4. Транспорт-тесты: 200 (admin), 403 (нет права), 422 (плохой диапазон),
   500 (ошибка сервиса).
5. ROADMAP: «Project Analytics — DONE 2026-08-15».
