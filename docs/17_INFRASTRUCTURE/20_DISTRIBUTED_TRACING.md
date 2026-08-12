# `19_INFRASTRUCTURE/20_DISTRIBUTED_TRACING.md`

```markdown
# STAIR PLATFORM

Document: 20_DISTRIBUTED_TRACING.md

ID: INFRA-0020

Status: APPROVED

---

# Purpose

Определяет Distributed Tracing Architecture.

---

# Trace Model

```text
Request
  │
  ▼
API
  │
  ├── Database
  │
  ├── Redis
  │
  ├── Queue
  │      │
  │      ▼
  │    Worker
  │      │
  │      ▼
  │    Engine
  │
  └── AI