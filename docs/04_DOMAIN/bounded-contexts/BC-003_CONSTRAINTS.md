# STAIR PLATFORM

**Bounded Context:** Constraints

**ID:** BC-003

**Type:** Core Engineering Domain

**Status:** APPROVED

---

# 1. Purpose

Constraints определяет все инженерные, геометрические, нормативные и производственные ограничения Stair Platform.

Контекст не изменяет модель.

Он только хранит правила, необходимые для проверки модели.

Constraints является единым источником инженерных ограничений для всех остальных движков платформы.

---

# 2. Responsibilities

Контекст отвечает за:

- хранение ограничений;
- классификацию ограничений;
- параметризацию ограничений;
- управление версиями правил;
- локализацию нормативов;
- предоставление правил другим контекстам.

---

# 3. Aggregate Root

```
ConstraintSet
```

ConstraintSet объединяет полный набор ограничений, применяемых к проекту.

---

# 4. Main Entities

- ConstraintSet
- Constraint
- RuleGroup
- RuleVersion
- StandardProfile

---

# 5. Value Objects

- ConstraintId
- RuleCode
- Tolerance
- Range
- AngleLimit
- HeightLimit
- WidthLimit
- LoadLimit
- MaterialRestriction

---

# 6. Constraint Categories

## Geometry

- минимальная высота ступени;
- максимальная высота ступени;
- минимальная глубина проступи;
- максимальный угол;
- минимальный просвет.

---

## Structural

- максимальный прогиб;
- допустимая нагрузка;
- коэффициент запаса;
- устойчивость.

---

## Manufacturing

- минимальная толщина материала;
- минимальный радиус гиба;
- размеры заготовки;
- ограничения оборудования.

---

## Safety

- строительные нормы;
- пожарные требования;
- требования по эвакуации;
- требования к ограждениям.

---

## Commercial

- минимальная партия;
- ограничения по материалам;
- региональные ограничения.

---

# 7. Domain Services

- ConstraintProvider
- ConstraintResolver
- RuleEngine
- StandardResolver

---

# 8. Domain Events

- ConstraintCreated
- ConstraintUpdated
- RuleActivated
- StandardChanged

---

# 9. Invariants

Всегда должны соблюдаться:

- активна только одна версия правила;
- правило принадлежит одному набору;
- правило имеет уникальный код;
- правило не может иметь пересекающиеся диапазоны.

---

# 10. Dependencies

Incoming

- Project

Outgoing

- Validation
- Solver
- Manufacturing
- Pricing

---

# 11. Public Interfaces

Контекст предоставляет:

- GetConstraintSet
- GetRule
- ResolveStandard
- ResolveTolerance

---

# 12. Policies

- Rule Version Policy
- Standard Policy
- Country Policy
- Manufacturing Policy

---

# 13. Future Extensions

Предусматривается поддержка:

- Eurocode;
- DIN;
- ISO;
- ГОСТ;
- ANSI;
- пользовательских корпоративных стандартов.

---

# 14. Approval

APPROVED