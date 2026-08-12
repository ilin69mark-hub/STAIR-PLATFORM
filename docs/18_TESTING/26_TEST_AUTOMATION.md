
---

# `20_TESTING/26_TEST_AUTOMATION.md`

```markdown
# STAIR PLATFORM

Document: 26_TEST_AUTOMATION.md

ID: TEST-0026

Status: APPROVED

---

# Purpose

Определяет Test Automation Strategy.

---

# Principle

Все повторяемые deterministic checks должны автоматизироваться.

---

# Automation Scope

Static Analysis

Unit Tests

Integration Tests

API Tests

Database Tests

Engine Tests

Geometry Tests

Security Tests

Contract Tests

Regression Tests

E2E Tests

Performance Tests

---

# CI Pipeline

```text
Commit
 ↓
Static Analysis
 ↓
Unit Tests
 ↓
Integration Tests
 ↓
Contract Tests
 ↓
Security Checks
 ↓
Build
 ↓
E2E
 ↓
Artifact