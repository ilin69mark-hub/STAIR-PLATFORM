# STAIR PLATFORM

Document: 17_GEOMETRY_VALIDATION.md

ID: ENG-GEO-0018

Status: APPROVED

---

# Purpose

Geometry Validation обеспечивает проверку корректности геометрической модели перед передачей данных в Solver, Manufacturing, Rendering и Export.

Validation гарантирует, что геометрия соответствует требованиям платформы и пригодна для дальнейшей обработки.

---

# Objectives

- обнаружение геометрических ошибок;
- обеспечение целостности модели;
- предотвращение передачи некорректной геометрии;
- формирование диагностического отчета.

---

# Validation Scope

## Topology

- Non-manifold edges
- Open shells
- Invalid faces
- Self-intersections
- Duplicate vertices
- Duplicate edges

---

## Solid

- Closed volume
- Positive volume
- Valid orientation
- Consistent normals
- Connected body

---

## Sketch

- Open contours
- Duplicate entities
- Invalid constraints
- Zero-length entities
- Overlapping geometry

---

## Parameters

- Invalid dimensions
- Invalid tolerances
- Missing references
- Out-of-range values

---

## Transformations

- Invalid matrices
- Degenerate transforms
- Broken coordinate systems

---

# Validation Levels

Quick

Standard

Full

Manufacturing

---

# Output

Validation Report содержит:

- Error ID
- Severity
- Geometry Element
- Description
- Suggested Fix

---

# Integration

Geometry Validation вызывается:

- после перестроения модели;
- перед экспортом;
- перед расчетами;
- перед генерацией производства.

---

# Acceptance Criteria

- автоматическая проверка геометрии;
- поддержка частичной проверки;
- генерация диагностического отчета.

---

APPROVED