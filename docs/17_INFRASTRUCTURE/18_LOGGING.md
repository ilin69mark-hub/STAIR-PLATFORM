
---

# `19_INFRASTRUCTURE/18_LOGGING.md`

```markdown id="k9y1h2"
# STAIR PLATFORM

Document: 18_LOGGING.md

ID: INFRA-0018

Status: APPROVED

---

# Purpose

Определяет Logging Architecture.

---

# Principle

Logs предназначены для operational visibility и investigation.

---

# Log Levels

DEBUG

INFO

WARN

ERROR

FATAL

---

# Structured Logging

Production logs должны иметь structured format.

---

# Required Metadata

Timestamp

Level

Service

Environment

Request ID

Trace ID where applicable

Job ID where applicable

Error Code where applicable

---

# Context

Для application events могут логироваться:

User ID where justified

Tenant ID

Project ID

Operation

Resource Type

---

# Forbidden Data

Не логируются:

Passwords

Tokens

API Keys

Secrets

Full Credentials

Sensitive Payloads

---

# Error Logging

Ошибки должны иметь:

Stable Error Code

Context

Correlation Information

Safe Message

---

# Centralization

Production logs должны централизованно собираться.

---

# Retention

Retention определяется operational и compliance requirements.

---

# Acceptance Criteria

По Request ID можно найти связанные application events без раскрытия sensitive data.

---

APPROVED
