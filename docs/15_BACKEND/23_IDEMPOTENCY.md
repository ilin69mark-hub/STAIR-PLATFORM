# STAIR PLATFORM

Document: 23_IDEMPOTENCY.md

ID: BE-0023

Status: APPROVED

---

# Purpose

Определяет механизм защиты от повторного выполнения операций.

---

# Required For

Payment

Commands

Job Creation

External Integrations

Document Generation

Manufacturing Operations

AI Mutations

---

# Idempotency Key

```text
Idempotency-Key
```

Ключ связывается с:

User

Tenant

Operation

Request

Result

---

# Flow

```text
Request
 ↓
Idempotency Check
 ↓
Existing?
 ├── Yes → Return Previous Result
 └── No
       ↓
    Execute
       ↓
    Store Result
```

---

# Rules

Повторная доставка одного Command не должна создавать дублирующее состояние.

---

# Job Idempotency

Job должна иметь уникальный execution identity.

---

# External Systems

При взаимодействии с внешним Provider используется provider-specific idempotency mechanism, если он поддерживается.

---

# Acceptance Criteria

Повторный запрос не приводит к повторному побочному эффекту.

---

APPROVED