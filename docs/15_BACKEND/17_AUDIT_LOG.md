# STAIR PLATFORM

Document: 17_AUDIT_LOG.md

ID: BE-0017

Status: APPROVED

---

# Purpose

Определяет систему аудита Backend.

Audit Log фиксирует значимые действия пользователей и системных компонентов.

---

# Audit Events

Login

Logout

Permission Change

Project Created

Project Updated

Project Deleted

Geometry Modified

Constraint Modified

Manufacturing Changed

Price Changed

Document Generated

AI Action

Administrative Action

---

# Audit Record

```text
Audit ID

Timestamp

Actor ID

Tenant ID

Project ID

Action

Resource Type

Resource ID

Result

Correlation ID

Metadata
```

---

# Immutability

Audit records не изменяются после создания.

Удаление допускается только в соответствии с установленной политикой хранения.

---

# AI Audit

AI-generated operations фиксируются отдельно с указанием:

AI Request

Command

Actor

Result

Affected Resources

---

# Rules

Audit Log не используется как основное бизнес-хранилище.

---

# Acceptance Criteria

Критические изменения системы можно восстановить по Audit Log.

---

APPROVED