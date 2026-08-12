# STAIR PLATFORM

Document: 07_AUDIT_MODEL.md

ID: DB-0007

Status: APPROVED

---

# Purpose

Определяет модель аудита инженерной платформы.

Audit обеспечивает трассируемость всех изменений.

---

# Audit Record

Audit ID

Timestamp

Actor

Action

Object ID

Revision

Changes

Source

Result

Correlation ID

---

# Audit Sources

User

API

AI

Scheduler

Import

System

---

# Logged Operations

Create

Update

Delete

Approve

Reject

Import

Export

Calculation

Generation

---

# Rules

Audit неизменяем.

Audit никогда не удаляется.

Audit связан с Revision.

---

# Storage

Audit хранится отдельно от Domain Tables.

---

# Search

Поиск поддерживается по:

Actor

Object

Revision

Date

Action

Correlation ID

---

# Acceptance Criteria

Каждое изменение журналируется.

---

APPROVED