# STAIR PLATFORM

Document: 27_RATE_LIMITING.md

ID: BE-0027

Status: APPROVED

---

# Purpose

Определяет ограничение нагрузки на Backend.

---

# Rate Limit Dimensions

IP

User

Tenant

API Key

Endpoint

Resource

Operation

---

# Examples

Authentication

Public API

AI API

Search

Export

Document Generation

---

# Architecture

```text
Request
 ↓
Rate Limiter
 ↓
Allowed?
 ├── No → 429
 └── Yes
       ↓
    Backend
```

---

# Burst

Поддерживается ограниченный burst для кратковременных пиков нагрузки.

---

# Distributed Rate Limiting

Для нескольких Backend instances используется shared state.

---

# Rules

Rate Limit не заменяет Authorization.

Внутренние trusted operations также должны иметь защиту от runaway workloads.

---

# Acceptance Criteria

Ни один отдельный клиент или процесс не может неконтролируемо потреблять ресурсы платформы.

---

APPROVED