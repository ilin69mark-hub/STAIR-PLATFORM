
---

# `20_TESTING/06_INTEGRATION_TESTING.md`

```markdown
# STAIR PLATFORM

Document: 06_INTEGRATION_TESTING.md

ID: TEST-0006

Status: APPROVED

---

# Purpose

Определяет Integration Testing Strategy.

---

# Scope

Integration Tests проверяют взаимодействие реальных компонентов.

---

# Targets

API + Database

Repository + Database

Worker + Queue

Engine + Storage

Engine + Database

Service + Service

Storage + Application

---

# Infrastructure

Для integration tests могут использоваться isolated instances:

PostgreSQL

Redis

Queue

Object Storage

---

# Test Lifecycle

```text
Start Dependencies
       ↓
Prepare Test Data
       ↓
Execute Test
       ↓
Verify Result
       ↓
Cleanup
       ↓
Destroy Dependencies