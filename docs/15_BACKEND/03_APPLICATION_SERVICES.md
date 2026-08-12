# STAIR PLATFORM

Document: 03_APPLICATION_SERVICES.md

ID: BE-0003

Status: APPROVED

---

# Purpose

Определяет Application Services.

Application Service реализует orchestration конкретного use case.

---

# Responsibilities

Validate Input

Load Context

Authorize

Execute Domain Logic

Invoke Engine

Persist Changes

Publish Events

---

# Example

```text
Create Stair Project

↓

Load User

↓

Validate Project

↓

Create Domain Aggregate

↓

Persist

↓

Initialize Graph

↓

Emit ProjectCreated

↓

Return Result
```

---

# Service Types

Project Service

Geometry Service

Graph Service

Manufacturing Service

Pricing Service

Document Service

AI Service

Search Service

---

# Rules

Application Service не содержит UI logic.

Application Service не зависит от конкретного HTTP framework.

---

# Acceptance Criteria

Use cases реализуются через независимые Application Services.

---

APPROVED