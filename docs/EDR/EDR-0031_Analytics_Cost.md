# STAIR PLATFORM

**Document:** EDR-0031_Analytics_Cost.md

**ID:** EDR-0031

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Analytics

---

# 1. Purpose

Документ фиксирует Cost Analytics (Phase F, F4) — четвёртый и последний
элемент Analytics bounded context (BC-043). Цель — «сколько стоит
производство и продажа»: агрегация финансовых метрик (себестоимость,
прибыль, налоги, итоговая цена) из снапшотов расчётов tenant за временное
окно. Read-only: источник — поле `result` таблицы `calculations` (JSONB,
`project.Snapshot`); данные парсятся на лету JSONB-функциями, новых
миграций и write-path-хуков нет (решение Phase F). Денежные значения —
минорные единицы (int64, ADR-0008 / EDR-0016 §3.3).

Состав:

1. **App-слой** — расширение `internal/application/analytics`:
   типы `CostPoint`, `CostTotals`, `CostReport`, метод `Service.Cost`.
2. **Репозиторий** — `internal/infrastructure/database/analytics_repo.go`:
   `CostTotals` и `CostSeries` (JSONB-агрегаты
   `result->'pricing'` + `generate_series`).
3. **Транспорт** — `GET /api/v1/admin/analytics/cost` (auth+admin,
   право `analytics.read`).
4. **Frontend** — подсекция «Стоимость» в AdminPanel.

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase F — Analytics: Cost Analytics)
- EDR-0028/EDR-0030 (Usage/Manufacturing Analytics — общая трубка:
  право, гранулярность, серии, JSONB-агрегаты)
- EDR-0029 (Project Analytics — паттерн агрегатов+отчёт)
- Pricing domain (`pricing.PriceBreakdown`, `Money` int64, `Currency`)
- ADR-0008 (Money: минорные единицы, валюта)

---

# 3. Model

## 3.1 Метрики

Источник — `calculations.result->'pricing'` (JSONB), структура
`pricing.PriceBreakdown` с PascalCase-ключами: `Currency.{Code,Decimals}`,
`Material`, `Machine`, `Labor`, `Overhead`, `ProductionCost`, `Margin`,
`Discount`, `PreTax`, `Tax`, `FinalPrice` (все `Money` = минорные единицы).

| Метрика | Определение |
|---------|-------------|
| `calculations` | Расчёты tenant в окне с полем `pricing` |
| `material` / `machine` / `labor` | Суммарные элементы себестоимости (минорные) |
| `production_cost` | Суммарная себестоимость (Material+Machine+Labor+Overhead) |
| `margin` / `discount` / `pre_tax` / `tax` | Суммарные элементы цепочки цены |
| `final_price` | Суммарная итоговая цена |
| `currency` | Валюта последних расчётов (единая для tenant) |
| `avg_final_price` | Средняя итоговая цена расчёта |

Серии по бакетам (`day|week|month`): `calculations`, `final_price`
(сумма), `avg_final_price`.

## 3.2 App-слой

```go
type CostPoint struct {
    Bucket         time.Time
    Calculations   int
    FinalPrice     int64 // сумма по бакету (минорные)
    AvgFinalPrice  float64
}

type CostTotals struct {
    Calculations   int
    Material       int64
    Machine        int64
    Labor          int64
    Overhead       int64
    ProductionCost int64
    Margin         int64
    Discount       int64
    PreTax         int64
    Tax            int64
    FinalPrice     int64
    AvgFinalPrice  float64
    Currency       string
}

type CostReport struct {
    From, To    time.Time
    Granularity Granularity
    Totals      CostTotals
    Series      []CostPoint
}
```

Порт `Repository` расширяется:

```go
CostTotals(ctx, tenantID string, from, to time.Time) (CostTotals, error)
CostSeries(ctx, tenantID string, from, to time.Time, g Granularity) ([]CostPoint, error)
```

Сервис: `Cost(ctx, tenantID, from, to, g) (*CostReport, error)` — валидация
диапазона/гранулярности как в EDR-0028.

## 3.3 SQL (репозиторий)

JSONB-агрегаты по расчётам tenant в окне (только с `pricing`):

```sql
SELECT COUNT(DISTINCT c.id),
       COALESCE(SUM(COALESCE((c.result->'pricing'->>'Material')::int8,0)),0),
       -- Machine, Labor, Overhead, Margin, Discount, PreTax, Tax, FinalPrice аналогично
       COALESCE(AVG(COALESCE((c.result->'pricing'->>'FinalPrice')::int8,0)),0)::float8,
       (SELECT (c2.result->'pricing'->'Currency'->>'Code')::text
         FROM calculations c2 JOIN public.projects p2 ON p2.id = c2.project_id
         WHERE p2.tenant_id = $1 AND c2.created_at BETWEEN $2 AND $3
           AND c2.result ? 'pricing'
         ORDER BY c2.created_at DESC LIMIT 1)
FROM calculations c JOIN public.projects p ON p.id = c.project_id
WHERE p.tenant_id = $1 AND c.created_at BETWEEN $2 AND $3
  AND c.result ? 'pricing';
```

Серии — `generate_series` + `date_trunc` + `LEFT JOIN` (пустые бакеты =
нули), как в EDR-0028 §3.3. Валюта берётся из последнего расчёта в окне
(предполагается единая для tenant).

## 3.4 Транспорт

`GET /api/v1/admin/analytics/cost?from&to&granularity`

- `from`/`to`/`granularity` — как в EDR-0028 (та же трубка `queryTime`,
  каталог гранулярностей).
- 200 — `CostReport`; 403 — нет права `analytics.read`; 422 —
  невалидные параметры; 500 — внутренняя ошибка.

## 3.5 Frontend

Подсекция «Стоимость» в AdminPanel: карточки `totals` (себестоимость по
элементам, цепочка цены, итоговая цена, средняя) + серия по бакетам.
Вызов `analyticsApi.cost(from, to, granularity)`.

---

# 4. Security

- Только admin: право `analytics.read` (единый путь `hasPermission`).
- Tenant-изоляция (SEC-0005): `calculations ⋈ projects` по `tenant_id`.
- Параметры валидируются (422), чтение не мутирует данные.

---

# 5. Acceptance

1. `go build ./...`, `go vet ./...`, `gofmt -w` — чисто.
2. Unit-тесты сервиса: валидация, проброс ошибок, сборка отчёта.
3. DB-тесты `analytics_repo_test`: totals и серия после расчётов с
   ценовым снапшотом; пустые бакеты — нули; изоляция tenant.
4. Транспорт-тесты: 200 (admin), 403, 422, 500.
5. ROADMAP: «Cost Analytics — DONE 2026-08-15»; Phase F — CLOSED.
