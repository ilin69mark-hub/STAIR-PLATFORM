
---

# `20_TESTING/04_TEST_ENVIRONMENTS.md`

```markdown
# STAIR PLATFORM

Document: 04_TEST_ENVIRONMENTS.md

ID: TEST-0004

Status: APPROVED

---

# Purpose

Определяет Testing Environments.

---

# Environments

Local

CI

Integration

Staging

Production Verification

---

# Local

Используется разработчиком.

Characteristics:

Fast

Isolated

Debuggable

---

# CI

Используется для автоматической проверки каждого изменения.

---

# Integration

Используется для tests с реальными infrastructure dependencies.

---

# Staging

Production-like environment.

Используется для:

E2E

Performance

Security

Release Validation

---

# Production Verification

Ограниченный набор безопасных проверок после deployment.

---

# Environment Isolation

Test environments не должны использовать Production Data.

---

# Database

Каждый test environment имеет собственную Database или изолированную schema/test database согласно конкретной testing strategy.

---

# Storage

Test files должны быть изолированы от Production Objects.

---

# Queue

Test jobs не должны попадать в Production Queue.

---

# External Providers

Используются:

Sandbox

Mock

Stub

Test Account

---

# Acceptance Criteria

Test execution не способен изменить Production Data.

---

APPROVED