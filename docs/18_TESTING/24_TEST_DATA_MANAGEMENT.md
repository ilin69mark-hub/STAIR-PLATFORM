
---

# `20_TESTING/24_TEST_DATA_MANAGEMENT.md`

```markdown
# STAIR PLATFORM

Document: 24_TEST_DATA_MANAGEMENT.md

ID: TEST-0024

Status: APPROVED

---

# Purpose

Определяет управление Test Data.

---

# Principles

Isolated

Reproducible

Minimal

Safe

Versioned where required

---

# Data Categories

Static Fixtures

Generated Data

Synthetic Data

Reference Data

Scenario Data

---

# Production Data

Production Data не должна использоваться в тестах без explicit anonymization и approval.

---

# Synthetic Data

Для большинства tests предпочтительно использовать synthetic data.

---

# Tenant Data

Test tenants должны быть полностью изолированы.

---

# Data Lifecycle

```text
Create
 ↓
Use
 ↓
Validate
 ↓
Cleanup