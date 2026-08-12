# STAIR PLATFORM

Document: 15_SECURITY_CONTEXT.md

ID: BE-0015

Status: APPROVED

---

# Purpose

Определяет Security Context Backend.

Security Context содержит информацию о текущем субъекте выполнения операции.

---

# Context

User ID

Tenant ID

Session ID

Roles

Permissions

Project ID

Correlation ID

Request ID

---

# Lifecycle

```text
Request
 ↓
Authentication
 ↓
Security Context
 ↓
Authorization
 ↓
Application
```

---

# Sources

JWT

Session

Service Identity

Internal Worker Identity

---

# Rules

Security Context создается на границе Backend.

Внутренние сервисы не должны самостоятельно извлекать identity из HTTP.

---

# Worker Context

Worker получает Security Context из Job Metadata либо Service Identity.

---

# Acceptance Criteria

Каждая защищенная операция выполняется в определенном Security Context.

---

APPROVED