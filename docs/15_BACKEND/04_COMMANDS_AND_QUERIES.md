# STAIR PLATFORM

Document: 04_COMMANDS_AND_QUERIES.md

ID: BE-0004

Status: APPROVED

---

# Purpose

Определяет Command/Query архитектуру Backend.

---

# Commands

Commands изменяют состояние.

```text
CreateProject
UpdateProject
DeleteProject

CreateGeometry
UpdateGeometry
DeleteGeometry

AddConstraint
RemoveConstraint

RunOptimization

GenerateDocument
CalculatePrice
```

---

# Queries

Queries читают состояние.

```text
GetProject
GetGeometry
GetGraph
GetManufacturingPlan
GetPrice
GetDocument
```

---

# Processing

```text
Command
 ↓
Handler
 ↓
Application Service
 ↓
Domain / Engine
 ↓
Repository
 ↓
Event
```

```text
Query
 ↓
Handler
 ↓
Read Model / Repository
 ↓
Response
```

---

# Rules

Commands являются изменяющими операциями.

Queries не изменяют состояние системы.

---

# Acceptance Criteria

Backend поддерживает предсказуемое разделение операций чтения и изменения.

---

APPROVED