# STAIR PLATFORM

Document: 01_ENGINE_ARCHITECTURE.md

ID: ENG-0002

Status: APPROVED

---

# Architecture

```

UI

↓

Application Layer

↓

Engineering Orchestrator

↓

Geometry Engine

↓

Constraint Engine

↓

Validation Engine

↓

Solver Engine

↓

Optimization Engine

↓

Manufacturing Engine

↓

Pricing Engine

↓

Rendering Engine

↓

Document Engine

↓

Infrastructure

```

---

# Rules

Engine не знает:

UI

HTTP

Database

ORM

Framework

---

Engine знает только:

Domain

Contracts

Events

Services

---

APPROVED