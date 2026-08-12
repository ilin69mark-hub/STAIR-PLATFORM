
---

# `20_TESTING/28_QUALITY_GATES.md`

```markdown
# STAIR PLATFORM

Document: 28_QUALITY_GATES.md

ID: TEST-0028

Status: APPROVED

---

# Purpose

Определяет Quality Gates STAIR PLATFORM.

---

# Principle

Quality Gate является формальной проверкой перед переходом системы на следующий lifecycle stage.

---

# Gate Levels

Development

Pull Request

Merge

Release

Production

---

# Development Gate

Проверяется:

Build

Unit Tests

Static Analysis

---

# Pull Request Gate

Проверяется:

Lint

Unit Tests

Integration Tests where applicable

Security Checks

Contract Checks

---

# Merge Gate

Проверяется:

Required Tests

No Critical Failures

No Unauthorized Dependency Changes

---

# Release Gate

Проверяется:

Critical Regression

E2E

Security

Performance where required

Database Migration

Infrastructure Compatibility

---

# Production Gate

Проверяется:

Deployment Health

Readiness

Critical API

Database

Queue

Workers

Engine

Observability

---

# Blocking Conditions

Release блокируется при:

Critical Test Failure

Critical Security Vulnerability

Broken Migration

Failed Critical E2E

Unacceptable Performance Regression

Failed Health Checks

---

# Exceptions

Exception допускается только при documented approval.

Exception должен иметь:

Reason

Risk

Owner

Expiration

Mitigation

---

# Quality Gate Flow

```text
Change
 ↓
Development Gate
 ↓
PR Gate
 ↓
Merge Gate
 ↓
Release Gate
 ↓
Production Gate