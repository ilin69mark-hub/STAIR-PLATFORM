# STAIR PLATFORM

**Document:** EDR-0002_Geometry_Limits.md

**ID:** EDR-0002

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-10

**Category:** Engineering

---

# 1. Purpose

Документ фиксирует нормативные ограничения геометрии лестничного марша
(категория Geometry, BC-003 Constraints).

Служит источником значений для Constraint Engine
(`internal/engine/constraint`).

---

# 2. Related Artifacts

- ADR-0008 (Engineering Coordinate and Units System)
- BC-003 (Constraints Bounded Context)
- EDR-0001 (Математическая модель расчёта марша)
- FR-100 (Проверка ограничений)

---

# 3. Constraints

## 3.1 Geometry

| Code | Constraint | Min | Max | Unit | Severity |
|------|-----------|-----|-----|------|----------|
| GEO-STEP-HEIGHT | высота ступени (riser) | 150 | 200 | mm | Error |
| GEO-TREAD-DEPTH | проступь (tread depth) | 260 | 320 | mm | Error |
| GEO-ANGLE | угол наклона марша | 30° | 45° | deg | Error |
| GEO-CLEARANCE | вертикальный просвет | 2000 | — | mm | Warning |
| GEO-STRINGER-THICKNESS | толщина косоура | 30 | — | mm | Warning |

## 3.2 Safety

| Code | Constraint | Min | Max | Unit | Severity |
|------|-----------|-----|-----|------|----------|
| SAF-RAILING-HEIGHT | высота ограждения | 900 | — | mm | Warning |

---

# 4. Range Semantics

- Диапазоны `Range{Tolerance}` не пересекаются (инвариант BC-003);
- одна активная версия правила (RuleVersion) — инвариант BC-003;
- уникальный RuleCode — инвариант BC-003;
- `Tolerance` применяется как допуск инженерной проверки (по умолчанию ±0.1 мм по ADR-0008).

---

# 5. Severity Model

| Severity | Значение | Переход в Solver |
|----------|----------|------------------|
| Error | нарушение обязательного норматива | запрещён |
| Warning | рекомендация / потенциальная проблема | разрешён |

См. EDR-0003 (правила валидации).

---

# 6. Standard Profile

По умолчанию активен профиль `STANDARD` (значения §3).

Будущие профили: Eurocode, DIN, ISO, ГОСТ, ANSI, корпоративные стандарты
(BC-003, recognize через StandardResolver).

---

# 7. Acceptance Criteria

- значения реализованы в `internal/engine/constraint` строго по §3;
- инварианты BC-003 проверяются тестами (непересекающиеся диапазоны,
  уникальный код, одна активная версия).

---

# 8. Changes

| Версия | Дата | Изменение |
|--------|------|-----------|
| 1.0.0 | 2026-08-10 | Первоначальная редакция |

---

APPROVED