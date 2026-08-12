
---

# `18_SECURITY/25_SECURITY_TESTING.md`

```markdown
# STAIR PLATFORM

Document: 25_SECURITY_TESTING.md

ID: SEC-0025

Status: APPROVED

---

# Purpose

Определяет Security Testing Strategy.

---

# Testing Areas

Authentication

Authorization

Tenant Isolation

Project Isolation

API Security

Input Validation

File Security

Database Security

Storage Security

AI Security

Integration Security

Secrets Management

---

# Test Types

Unit Security Tests

Integration Security Tests

API Security Tests

E2E Security Tests

Static Analysis

Dependency Scanning

Dynamic Security Testing

Penetration Testing

---

# Authorization Tests

Проверяются:

Unauthorized Access

Cross-Tenant Access

Privilege Escalation

Resource Enumeration

Permission Bypass

---

# API Tests

Проверяются:

Authentication Bypass

Injection

Malformed Requests

Rate Limit Bypass

Oversized Payloads

Replay

---

# File Tests

Проверяются:

Malicious Files

Path Traversal

Archive Bomb

Invalid Content Type

Parser Abuse

---

# AI Security Tests

Проверяются:

Prompt Injection

Tool Abuse

Permission Bypass

Context Leakage

Cross-Tenant Context Leakage

Unauthorized Mutation

---

# Regression

Security tests должны выполняться после изменений критических security components.

---

# Acceptance Criteria

Критические Security Controls имеют автоматизированные regression tests.

---

APPROVED