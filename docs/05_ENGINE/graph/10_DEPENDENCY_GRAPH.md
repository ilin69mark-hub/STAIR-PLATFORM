# STAIR PLATFORM

Document: 02_DEPENDENCY_GRAPH.md

ID: ENG-GEO-0003

Status: APPROVED

---

# Purpose

Dependency Graph хранит зависимости между всеми объектами модели.

---

# Node Types

Project

Assembly

Component

Sketch

Constraint

Parameter

Reference Plane

Coordinate System

Material

---

# Edge Types

Depends On

Uses

Constrains

References

Owns

Connected To

---

# Rules

Изменение Parameter

↓

Обновляет только зависимые узлы

↓

Не перестраивает всю модель

---

# Benefits

Incremental Rebuild

Undo

AI Navigation

Fast Solver

Fast Export

---

APPROVED