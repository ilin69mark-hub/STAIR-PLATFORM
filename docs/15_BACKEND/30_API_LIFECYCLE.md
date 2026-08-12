# STAIR PLATFORM

Document: 30_API_LIFECYCLE.md

ID: BE-0030

Status: APPROVED

---

# Purpose

Определяет жизненный цикл Backend API Request.

---

# Lifecycle

```text
Request
 ↓
Receive
 ↓
Context Creation
 ↓
Authentication
 ↓
Authorization
 ↓
Rate Limit
 ↓
Validation
 ↓
Idempotency
 ↓
Application
 ↓
Persistence / Engine
 ↓
Event
 ↓
Response        