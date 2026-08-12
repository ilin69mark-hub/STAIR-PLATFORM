# STAIR PLATFORM

Document: 16_GEOMETRY_EVENTS.md

ID: ENG-GEO-0017

Status: APPROVED

---

# Purpose

Документ определяет события Geometry Platform.

---

# Events

GeometryCreated

GeometryUpdated

GeometryDeleted

TopologyBuilt

SolidBuilt

MeshGenerated

BooleanOperationCompleted

TransformationApplied

MeasurementCalculated

GeometryImported

GeometryExported

GeometryCacheInvalidated

---

# Rules

Все события:

- immutable;
- versioned;
- traceable;
- correlated с Revision проекта.

---

APPROVED