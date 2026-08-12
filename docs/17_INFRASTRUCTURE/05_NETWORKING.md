# STAIR PLATFORM

Document: 05_NETWORKING.md

ID: INFRA-0005

Status: APPROVED

---

# Purpose

Определяет Network Architecture.

---

# Network Zones

```text
Internet
   │
   ▼
Public Edge
   │
   ▼
Application Network
   │
   ├── API
   ├── Workers
   ├── Engine
   └── AI
   │
   ▼
Private Data Network
   ├── PostgreSQL
   ├── Redis
   ├── Queue
   └── Storage