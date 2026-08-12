# STAIR PLATFORM

Document: 01_PARAMETRIC_MODEL.md

ID: ENG-GEO-0002

Status: APPROVED

---

# Purpose

Parametric Model хранит исключительно параметры изделия.

Все остальные данные вычисляются.

---

# Parameter Groups

Project

↓

Geometry

↓

Materials

↓

Loads

↓

Manufacturing

↓

Rendering

---

# Geometry Parameters

Width

Height

Length

Angle

Step Count

Step Height

Step Width

Landing Width

Stringer Thickness

Material

Finish

Tolerance

---

# Rules

Изменение любого параметра

↓

Invalidates Dependency Graph

↓

Triggers Partial Rebuild

---

APPROVED