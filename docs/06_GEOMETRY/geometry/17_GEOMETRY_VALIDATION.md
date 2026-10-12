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

**Состояние проверено по коду `internal/geometry/validate.go` 2026-10-12.**
Ниже отмечено, что действительно проверяется, а что заявлено, но не
реализовано. Проверка самопересечений требует карты пересечений — это
отдельная работа (бэклог S-171).

## Topology

| Проверка | Состояние | Код |
|---|---|---|
| Non-manifold edges | РЕАЛИЗОВАНА | `GEO-SOLID-NON-MANIFOLD` — ребро принадлежит ровно двум граням |
| Open shells | РЕАЛИЗОВА (частично) | `GEO-FACE-OPEN-WIRE` — незамкнутость контура грани |
| Invalid faces | РЕАЛИЗОВАНА | `GEO-FACE-FEW-EDGES` — менее трёх рёбер |
| **Self-intersections** | **НЕ РЕАЛИЗОВАНА** | нет ни проверки пересечения граней, ни карты пересечений тел |
| Duplicate vertices | Не проверяется | чинится на этапе построения, а не проверкой |
| Duplicate edges | Не проверяется | — |

## Solid

| Проверка | Состояние | Код |
|---|---|---|
| Closed volume | РЕАЛИЗОВАНА | `GEO-SOLID-NON-POSITIVE-VOLUME` |
| Positive volume | РЕАЛИЗОВАНА | там же |
| Valid orientation | Косвенно | ловится только фактом объёма ≤ 0; локализовать грань нельзя |
| Consistent normals | Не проверяется | — |
| Connected body | Не проверяется | — |

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