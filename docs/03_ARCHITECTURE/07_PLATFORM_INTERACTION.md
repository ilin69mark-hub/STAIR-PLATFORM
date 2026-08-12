# STAIR PLATFORM

Document: 07_PLATFORM_INTERACTION.md

ID: ARCH-0008

Status: APPROVED

---

# Purpose

Документ определяет допустимые способы взаимодействия между платформами STAIR Platform.

---

# Interaction Principles

Platform-to-Platform

API First

Event Driven

Loose Coupling

Contract First

---

# Allowed Communication

REST API

GraphQL

WebSocket

Event Bus

Background Jobs

Shared Contracts

---

# Forbidden Communication

Database-to-Database

Shared Internal Models

Shared Repositories

Direct Internal Calls

Hidden Dependencies

---

# Interaction Flow

```
Client

↓

API Platform

↓

Business Platform

↓

Event Platform

↓

Subscribers
```

---

# Cross Platform Rules

Geometry не обращается напрямую к Manufacturing.

Manufacturing не обращается к Pricing Database.

Pricing использует только API или события.

AI использует только Canonical DTO.

---

# Acceptance Criteria

- отсутствуют прямые зависимости;
- все взаимодействие проходит через утвержденные каналы;
- соблюдены архитектурные границы.

---

APPROVED