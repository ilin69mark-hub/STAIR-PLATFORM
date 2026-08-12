# STAIR PLATFORM

Document: 16_PERMISSION_ENFORCEMENT.md

ID: BE-0016

Status: APPROVED

---

# Purpose

Определяет применение Authorization в Backend.

---

# Authorization Layers

```text
Authentication
 ↓
Identity
 ↓
Role
 ↓
Permission
 ↓
Resource Policy
 ↓
Operation
```

---

# Permission Types

Project Read

Project Write

Geometry Edit

Graph Read

Manufacturing Manage

Pricing Manage

Document Generate

AI Execute

Administration

---

# Resource Authorization

Проверяется:

User

Tenant

Project

Resource

Operation

---

# Rules

Frontend visibility не является Authorization.

Backend является конечной точкой enforcement.

---

# Engine Authorization

Engine операции также проходят Authorization.

AI не получает отдельные обходные права.

---

# Acceptance Criteria

Ни одна защищенная операция не может быть выполнена без соответствующего Permission.

---

APPROVED