
---

# `20_TESTING/20_RELIABILITY_TESTING.md`

```markdown
# STAIR PLATFORM

Document: 20_RELIABILITY_TESTING.md

ID: TEST-0020

Status: APPROVED

---

# Purpose

Определяет Reliability Testing Strategy.

---

# Objective

Проверить способность системы продолжать работу при отказах компонентов.

---

# Failure Scenarios

Service Failure

Instance Failure

Database Connection Failure

Cache Failure

Queue Failure

Storage Failure

External API Failure

AI Provider Failure

Network Failure

---

# Recovery

```text
Failure
 ↓
Detection
 ↓
Isolation
 ↓
Fallback
 ↓
Recovery
 ↓
Validation