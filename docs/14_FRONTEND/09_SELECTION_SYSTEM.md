# STAIR PLATFORM

Document: 09_SELECTION_SYSTEM.md

ID: FE-0009

Status: APPROVED

---

# Purpose

Определяет систему выбора объектов инженерной модели.

Selection System является единой точкой выбора элементов платформы.

---

# Selectable Objects

Project

Assembly

Part

Sketch

Edge

Face

Vertex

Constraint

Dimension

Graph Node

Document

---

# Selection Types

Single

Multiple

Area

Hierarchy

Filter

---

# Selection State

Hovered

Selected

Focused

Locked

Hidden

---

# Selection Events

Select

Deselect

Toggle

Focus

Highlight

---

# Rules

В системе существует только один глобальный Selection Context.

Все модули используют общий Selection API.

---

# Acceptance Criteria

Любой объект платформы выбирается единым механизмом.

---

APPROVED