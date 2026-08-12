
---

# `19_INFRASTRUCTURE/07_SERVICE_DISCOVERY.md`

```markdown
# STAIR PLATFORM

Document: 07_SERVICE_DISCOVERY.md

ID: INFRA-0007

Status: APPROVED

---

# Purpose

Определяет механизм обнаружения внутренних services.

---

# Principle

Services не должны зависеть от hardcoded IP addresses.

---

# Service Identity

Каждый internal service имеет:

Service Name

Environment

Port

Protocol

Health State

---

# Discovery Flow

```text
Service A
   ↓
Service Discovery
   ↓
Service B