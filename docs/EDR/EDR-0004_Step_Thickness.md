# STAIR PLATFORM

**Document:** EDR-0004_Step_Thickness.md

**ID:** EDR-0004

**Status:** APPROVED

**Author:** Project Team

**Date:** 2026-08-11

**Category:** Engineering

---

# 1. Purpose

Документ фиксирует введение доменного параметра `StepThickness`
(толщина проступи/подступенка) для Geometry Engine (ENG-GEO-0007).
Параметр отсутствует в STAIR-DOC; добавлен как расширение параметрической
модели по решению Geometry MVP-04.

---

# 2. Related Artifacts

- ADR-0008 (Engineering Coordinate and Units System)
- BC-002 (Geometry Invariants)
- ENG-GEO-0007 (Solid Builder)
- StairConfiguration (`internal/domain/engineering/stair.go`)

---

# 3. Model

```text
StepThickness : Length   // толщина проступи/подступенка, мм (ADR-0008)
```

- Параметр входит в `StairConfiguration`;
- инвариант: `StepThickness >= 0` (BC-002 geometry invariants, not negative),
  проверяется в `Validate()`;
- семантика: высота экструзии проступи вверх (ось Z) и толщина
  подступенка по оси X в B-Rep модели марша;
- параметр является единственным источником истины геометрии ступени
  (BC-002); mesh — производная величина и не хранится как истина.

---

# 4. Defaults

- При `StepThickness == 0` геометрия ступеней не строится (пустые тела
  не создаются); положительное значение задаётся пользователем.

---

# 5. Tests

- `Validate()` отклоняет отрицательное значение;
- builder (ENG-GEO-0007) использует значение для экструзии ступеней.

---

# 6. Changes

| Версия | Дата | Изменение |
|--------|------|-----------|
| 1.0.0 | 2026-08-11 | Первоначальная редакция |

---

APPROVED
