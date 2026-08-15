# STAIR PLATFORM

**Document:** EDR-0030_Analytics_Manufacturing.md

**ID:** EDR-0030

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-15

**Category:** Analytics

---

# 1. Purpose

Документ фиксирует Manufacturing Analytics (Phase F, F3) — третий элемент
Analytics bounded context (BC-043). Цель — «сколько и как производится»:
агрегация производственных данных (детали, BOM, карта раскроя, раскрой)
из снапшотов расчётов tenant за временное окно. Read-only: источник —
поле `result` таблицы `calculations` (JSONB, полный снапшот
`project.Snapshot`); данные парсятся на лету JSONB-функциями, новых
миграций и write-path-хуков нет (решение Phase F).

Состав:

1. **App-слой** — расширение `internal/application/analytics`:
   типы `ManufacturingPoint`, `ManufacturingTotals`, `ManufacturingReport`,
   метод `Service.Manufacturing`.
2. **Репозиторий** — `internal/infrastructure/database/analytics_repo.go`:
   `ManufacturingTotals` и `ManufacturingSeries` (JSONB-агрегаты
   `result->'manufacturing'` + `generate_series`).
3. **Транспорт** — `GET /api/v1/admin/analytics/manufacturing` (auth+admin,
   право `analytics.read`).
4. **Frontend** — подсекция «Производство» в AdminPanel.

---

# 2. Related Artifacts

- ROADMAP-0011 (Phase F — Analytics: Manufacturing Analytics)
- EDR-0028 (Usage Analytics — общая трубка: право, гранулярность, серии)
- EDR-0029 (Project Analytics — агрегаты+отчёт, тот же паттерн)
- EDR-0022 (CAD Export) / ADR-0008 — производственный снапшот
- BC-007 / Manufacturing domain (`ManufacturingPackage`)

---

# 3. Model

## 3.1 Метрики

Источник — `calculations.result->'manufacturing'` (JSONB), структура
`manufacturing.ManufacturingPackage` с PascalCase-ключами:
`Parts`, `BOM.Lines`, `CutList.Items`, `Nesting.{Sheets,PartCount,PartArea,
SheetArea,WasteArea,Utilization}`.

| Метрика | Определение |
|---------|-------------|
| `calculations` | Расчёты tenant в окне с полем `manufacturing` |
| `parts` | Суммарное число деталей (`Parts` длина по всем расчётам) |
| `bom_lines` | Суммарное число строк BOM |
| `cut_items` | Суммарное число позиций карты раскроя |
| `sheets` | Суммарное число листов раскроя |
| `part_area` / `sheet_area` / `waste_area` | Суммарные площади (мм²) |
| `utilization` | Средняя утилизация листа (0..1) |
| `materials` | `{material: parts}` — распределение деталей по материалам |

Серии по бакетам (`day|week|month`): `calculations`, `parts`, `sheets`,
`utilization` (средняя).

## 3.2 App-слой

```go
type ManufacturingPoint struct {
    Bucket       time.Time
    Calculations int
    Parts        int
    Sheets       int
    Utilization  float64 // средняя по бакету
}

type ManufacturingTotals struct {
    Calculations int
    Parts        int
    BomLines     int
    CutItems     int
    Sheets       int
    PartArea     float64
    SheetArea    float64
    WasteArea    float64
    Utilization  float64
    Materials    map[string]int
}

type ManufacturingReport struct {
    From, To    time.Time
    Granularity Granularity
    Totals      ManufacturingTotals
    Series      []ManufacturingPoint
}
```

Порт `Repository` расширяется:

```go
ManufacturingTotals(ctx, tenantID string, from, to time.Time) (ManufacturingTotals, error)
ManufacturingSeries(ctx, tenantID string, from, to time.Time, g Granularity) ([]ManufacturingPoint, error)
```

Сервис: `Manufacturing(ctx, tenantID, from, to, g) (*ManufacturingReport, error)`
— валидация диапазона/гранулярности как в EDR-0028.

## 3.3 SQL (репозиторий)

JSONB-агрегаты по расчётам tenant в окне (только с `manufacturing`):

```sql
SELECT COUNT(DISTINCT c.id),
       COALESCE(SUM(jsonb_array_length(COALESCE(c.result->'manufacturing'->'Parts','[]'::jsonb))),0),
       -- BOM.Lines, CutList.Items, Nesting.Sheets аналогично
       COALESCE(SUM(COALESCE((c.result->'manufacturing'->'Nesting'->>'PartArea')::float8,0)),0),
       ...
       COALESCE(AVG(COALESCE((c.result->'manufacturing'->'Nesting'->>'Utilization')::float8,0)),0)
FROM calculations c JOIN public.projects p ON p.id = c.project_id
WHERE p.tenant_id = $1 AND c.created_at BETWEEN $2 AND $3
  AND c.result ? 'manufacturing';
```

Распределение по материалам — отдельным запросом (`jsonb_array_elements`
по `Parts`, группировка по `->>'Material'`). Серии — `generate_series` +
`date_trunc` + `LEFT JOIN` (пустые бакеты = нули), как в EDR-0028 §3.3.

Пустые/нулевые `manufacturing` не учитываются (кроме `calculations`, где
условие «есть ключ»). COALESCE защищает от `null`-массивов.

## 3.4 Транспорт

`GET /api/v1/admin/analytics/manufacturing?from&to&granularity`

- `from`/`to`/`granularity` — как в EDR-0028 (та же трубка `queryTime`,
  каталог гранулярностей).
- 200 — `ManufacturingReport`; 403 — нет права `analytics.read`; 422 —
  невалидные параметры; 500 — внутренняя ошибка.

## 3.5 Frontend

Подсекция «Производство» в AdminPanel: карточки `totals` (расчёты, детали,
BOM, карта раскроя, листы, утилизация, распределение по материалам) +
серия по бакетам. Вызов `analyticsApi.manufacturing(from, to, granularity)`.

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
   производственным снапшотом; пустые бакеты — нули; изоляция tenant.
4. Транспорт-тесты: 200 (admin), 403, 422, 500.
5. ROADMAP: «Manufacturing Analytics — DONE 2026-08-15».
