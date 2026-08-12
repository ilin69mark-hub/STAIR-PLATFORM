
---

# `20_TESTING/21_REGRESSION_TESTING.md`

```markdown
# STAIR PLATFORM

Document: 21_REGRESSION_TESTING.md

ID: TEST-0021

Status: APPROVED

---

# Purpose

Определяет Regression Testing Strategy.

---

# Objective

Предотвращать повторное появление ранее исправленных defects.

---

# Regression Sources

Bug Fixes

Architecture Changes

Domain Changes

Engine Changes

Geometry Changes

API Changes

Database Changes

Security Changes

Infrastructure Changes

---

# Regression Case

Каждый critical defect после исправления должен получить соответствующий regression test.

---

# Regression Suite

```text
Critical
   ↓
High
   ↓
Standard