# STAIR PLATFORM

Document: 11_TRANSFORMATION_ENGINE.md

ID: ENG-GEO-0012

Status: APPROVED

---

# Purpose

Transformation Engine отвечает за пространственные преобразования объектов.

---

# Supported Transformations

Translate

Rotate

Scale

Mirror

Align

Pattern

Array

Move To Coordinate System

---

# Coordinate Spaces

Global

Local

Assembly

Work

User

---

# Rules

Преобразования не изменяют исходную параметрическую модель.

Все операции обратимы.

История преобразований журналируется.

---

# Acceptance Criteria

- Поддержка вложенных преобразований.
- Работа через матрицы преобразований.
- Совместимость с Graph Platform.

---

APPROVED