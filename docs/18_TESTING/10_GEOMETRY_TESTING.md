
---

# `20_TESTING/10_GEOMETRY_TESTING.md`

```markdown
# STAIR PLATFORM

Document: 10_GEOMETRY_TESTING.md

ID: TEST-0010

Status: APPROVED

---

# Purpose

Определяет Testing Strategy Geometry Layer.

---

# Scope

Parametric Model

Coordinate System

Topology

Sketch

Constraint Solver

Solid Builder

Mesh Builder

Boolean Operations

Transformations

Measurements

Feature Tree

Geometry History

---

# Geometry Correctness

Проверяется:

Dimensions

Coordinates

Topology

Connectivity

Orientation

Validity

---

# Parametric Testing

Изменение parameter должно приводить к ожидаемому изменению geometry.

---

# Constraint Testing

Проверяются:

Valid Constraint

Conflicting Constraint

Under-Constrained Model

Over-Constrained Model

Solved Model

---

# Boolean Testing

Проверяются:

Union

Difference

Intersection

Invalid Geometry

Touching Geometry

Coincident Geometry

---

# Transformation Testing

Проверяются:

Translation

Rotation

Scaling where applicable

Coordinate Transformation

---

# Measurement Testing

Проверяются:

Length

Angle

Area

Volume

Distance

Bounding Box

---

# Numerical Precision

Тесты должны учитывать установленную tolerance policy.

---

# Regression Geometry

Reference geometry может использоваться для сравнения результатов.

---

# Golden Tests

Для сложных deterministic geometry operations могут применяться golden/reference results.

---

# Acceptance Criteria

Geometry Engine не принимает некорректную geometry как valid result без explicit error/state.

---

APPROVED