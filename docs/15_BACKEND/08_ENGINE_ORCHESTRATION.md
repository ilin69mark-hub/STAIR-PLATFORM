# STAIR PLATFORM

Document: 08_ENGINE_ORCHESTRATION.md

ID: BE-0008

Status: APPROVED

---

# Purpose

Определяет взаимодействие Backend с Engine Layer.

Backend координирует Engine, но не заменяет его.

---

# Engine Components

Geometry Engine

Graph Engine

Constraint Solver

Optimization Engine

Manufacturing Engine

Pricing Engine

---

# Invocation

```text
Application Service
        ↓
Engine Interface
        ↓
Engine Runtime
        ↓
Calculation
        ↓
Engine Result
```

---

# Engine Request

Engine Request содержит:

Project Context

Entity IDs

Parameters

Constraints

Operation

Correlation ID

---

# Engine Result

Engine Result содержит:

Status

Result

Changes

Errors

Warnings

Metrics

---

# Rules

Backend не выполняет инженерные вычисления.

Engine не зависит от HTTP Layer.

---

# Long Running Operations

Длительные Engine операции выполняются через Job System.

---

# Acceptance Criteria

Engine может использоваться независимо от API.

---

APPROVED