
---

# `20_TESTING/25_TEST_FIXTURES.md`

```markdown
# STAIR PLATFORM

Document: 25_TEST_FIXTURES.md

ID: TEST-0025

Status: APPROVED

---

# Purpose

Определяет правила создания и использования Test Fixtures.

---

# Fixture

Fixture — заранее определенное состояние или набор данных, необходимый для выполнения теста.

---

# Fixture Types

Entity Fixture

Database Fixture

Geometry Fixture

Project Fixture

User Fixture

Tenant Fixture

File Fixture

Engine Fixture

---

# Principles

Fixtures должны быть:

Minimal

Explicit

Reusable

Deterministic

---

# Example

```text
Tenant
 └── User
      └── Project
           └── Stair Configuration
                └── Geometry