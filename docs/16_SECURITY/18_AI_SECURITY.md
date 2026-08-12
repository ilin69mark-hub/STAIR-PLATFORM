
---

# `18_SECURITY/18_AI_SECURITY.md`

```markdown
# STAIR PLATFORM

Document: 18_AI_SECURITY.md

ID: SEC-0018

Status: APPROVED

---

# Purpose

Определяет Security Model AI Layer.

---

# Principle

AI является недоверенным reasoning component.

AI не получает прямой unrestricted access к системе.

---

# AI Boundary

```text
User
 ↓
AI API
 ↓
AI Runtime
 ↓
Context
 ↓
Model
 ↓
Tool / Command
 ↓
Authorization
 ↓
Application
 ↓
Domain / Engine