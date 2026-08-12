# STAIR PLATFORM

Document: 04_CONSTRAINT_SOLVER.md

ID: ENG-GEO-0104

Status: APPROVED

---

# Purpose

Constraint Solver вычисляет систему геометрических ограничений.

---

# Responsibilities

Построение графа ограничений.

Поиск конфликтов.

Обнаружение переопределенных связей.

Обнаружение недоопределенных эскизов.

Автоматическое распространение изменений.

---

# Solver Pipeline

Constraints

↓

Dependency Graph

↓

Solve

↓

Validate

↓

Update Geometry

---

# Rules

Решение должно быть детерминированным.

Циклические зависимости запрещены.

Все конфликты сопровождаются диагностикой.

---

# Acceptance Criteria

- Решение больших систем ограничений.
- Диагностика конфликтов.
- Поддержка частичного пересчета.

APPROVED