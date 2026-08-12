# STAIR PLATFORM

Document: 22_VALIDATION.md

ID: BE-0022

Status: APPROVED

---

# Purpose

Определяет систему валидации Backend.

---

# Validation Layers

```text
Transport Validation
        ↓
Application Validation
        ↓
Domain Validation
        ↓
Persistence Validation
```

---

# Transport Validation

Проверяет:

Format

Required Fields

Type

Size

Syntax

---

# Application Validation

Проверяет:

Use Case Preconditions

Permissions

Resource Availability

Workflow State

---

# Domain Validation

Проверяет:

Business Rules

Invariants

Constraints

Aggregate Rules

---

# Persistence Validation

Проверяет:

Database Constraints

Uniqueness

Foreign Keys

Data Integrity

---

# Rules

Validation не заменяет Authorization.

Domain Validation не зависит от HTTP.

---

# Acceptance Criteria

Некорректная операция отклоняется до изменения состояния системы.

---

APPROVED