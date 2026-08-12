
---

# `18_SECURITY/06_PROJECT_SECURITY.md`

```markdown
# STAIR PLATFORM

Document: 06_PROJECT_SECURITY.md

ID: SEC-0006

Status: APPROVED

---

# Purpose

Определяет безопасность Project как основной рабочей области STAIR PLATFORM.

---

# Project Boundary

Project принадлежит Tenant и имеет собственную Resource Policy.

```text
Tenant
  │
  └── Project
       ├── Geometry
       ├── Graph
       ├── Manufacturing
       ├── Pricing
       ├── Documents
       └── AI Context