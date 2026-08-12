# STAIR PLATFORM

Document: 02_APPLICATION_LAYER.md

ID: BE-0002

Status: APPROVED

---

# Purpose

Определяет Application Layer Backend.

Application Layer реализует use cases и координирует выполнение операций.

---

# Responsibilities

Use Cases

Command Handling

Workflow Orchestration

Transaction Management

Authorization Context

Domain Coordination

Engine Coordination

Event Publication

---

# Structure

```text
Application
│
├── Commands
├── Queries
├── Services
├── Handlers
├── DTO
└── Workflows
```

---

# Command

Command изменяет состояние системы.

Examples:

CreateProject

UpdateGeometry

AddConstraint

RunCalculation

GenerateDocument

CreateManufacturingOrder

---

# Query

Query получает данные без изменения состояния.

Examples:

GetProject

GetGeometry

GetGraph

GetPrice

SearchProject

---

# Rules

Command и Query не смешиваются без необходимости.

Application Layer не содержит низкоуровневой persistence logic.

---

# Acceptance Criteria

Все пользовательские use cases проходят через Application Layer.

---

APPROVED