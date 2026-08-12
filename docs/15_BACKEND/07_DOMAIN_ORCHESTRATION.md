# STAIR PLATFORM

Document: 07_DOMAIN_ORCHESTRATION.md

ID: BE-0007

Status: APPROVED

---

# Purpose

Определяет взаимодействие Backend с Domain Layer.

---

# Responsibilities

Load Aggregate

Validate Command

Execute Domain Operation

Persist Aggregate

Publish Domain Events

---

# Flow

```text
Application Service
        ↓
Load Aggregate
        ↓
Domain Operation
        ↓
Domain Events
        ↓
Repository
        ↓
Event Publisher
```

---

# Domain Boundary

Backend не переносит Domain Logic в Application Service.

Application Service только координирует выполнение.

---

# Aggregate Rules

Aggregate изменяется через определенные операции.

Прямое изменение внутренних полей Aggregate запрещено.

---

# Validation

Application Validation

↓

Domain Validation

↓

Persistence Validation

---

# Acceptance Criteria

Business Rules остаются внутри Domain Layer.

---

APPROVED