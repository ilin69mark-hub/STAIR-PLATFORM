# STAIR PLATFORM

Document: 02_ENGINE_PIPELINE.md

ID: ENG-0003

Status: APPROVED

---

# Pipeline

```

User Action

↓

Project

↓

Geometry

↓

Constraints

↓

Validation

↓

Solver

↓

Optimization

↓

Manufacturing

↓

Pricing

↓

Rendering

↓

Documents

↓

Completed

```

---

# Restart Rules

Изменение Geometry

↓

Validation

↓

Solver

↓

Optimization

↓

Manufacturing

↓

Pricing

↓

Rendering

---

Изменение стоимости НЕ запускает Solver.

Изменение материалов запускает Solver.

---

# Cache Rules

Каждый Engine обязан использовать результаты предыдущего вычисления при неизменных входных данных.

---

APPROVED