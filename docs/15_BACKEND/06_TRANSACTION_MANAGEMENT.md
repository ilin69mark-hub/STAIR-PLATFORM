# STAIR PLATFORM

Document: 06_TRANSACTION_MANAGEMENT.md

ID: BE-0006

Status: APPROVED

---

# Purpose

Определяет управление транзакциями Backend.

---

# Transaction Boundary

Transaction определяется Application Use Case.

---

# Lifecycle

```text
Begin
 ↓
Execute
 ↓
Validate
 ↓
Persist
 ↓
Publish Transactional Events
 ↓
Commit
```

При ошибке:

```text
Rollback
```

---

# Responsibilities

Atomicity

Consistency

Isolation

Rollback

Concurrency Control

---

# Transaction Types

Short Transaction

Long Running Job

Distributed Workflow

---

# Long Running Operations

Длительные операции не должны удерживать Database Transaction.

Используется Job/Workflow подход.

---

# Retry

Retry применяется только для операций, которые безопасно повторять.

---

# Rules

Transaction не управляется UI.

Transaction не определяется HTTP Handler напрямую.

---

# Acceptance Criteria

Каждый изменяющий Use Case имеет явно определенную Transaction Boundary.

---

APPROVED