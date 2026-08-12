# STAIR PLATFORM

Document: 28_REQUEST_CONTEXT.md

ID: BE-0028

Status: APPROVED

---

# Purpose

Определяет единый Request Context Backend.

---

# Context Fields

Request ID

Correlation ID

Trace ID

User ID

Tenant ID

Project ID

Session ID

Deadline

Cancellation

Locale

---

# Lifecycle

```text
Request
 ↓
Context Creation
 ↓
API Adapter
 ↓
Application
 ↓
Domain / Engine
 ↓
Infrastructure
```

---

# Propagation

Context передается между:

API

Application

Workers

Events

Engine

Database

External Integrations

---

# Cancellation

При отмене пользовательского запроса долгие операции должны корректно завершаться либо переходить в Background Job.

---

# Rules

Context не содержит бизнес-состояние.

Context используется только для execution metadata и lifecycle control.

---

# Acceptance Criteria

Request может быть прослежен через все связанные Backend операции.

---

APPROVED