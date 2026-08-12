
---

# `20_TESTING/01_TESTING_STRATEGY.md`

```markdown
# STAIR PLATFORM

Document: 01_TESTING_STRATEGY.md

ID: TEST-0001

Status: APPROVED

---

# Purpose

Определяет общую Testing Strategy.

---

# Strategy

Testing строится на четырех уровнях:

1. Verification
2. Validation
3. Regression
4. Production Verification

---

# Verification

Проверяет соответствие implementation определенным требованиям.

Примеры:

Unit Tests

Static Analysis

Type Checks

Schema Validation

---

# Validation

Проверяет соответствие системы реальному ожидаемому behavior.

Примеры:

Integration Tests

System Tests

Acceptance Tests

---

# Regression

Проверяет отсутствие поломок ранее работающего поведения.

---

# Production Verification

После deployment проверяются:

Health

Availability

Critical API

Database Connectivity

Queue

Core Engine

---

# Risk-Based Testing

Test coverage определяется не только количеством строк кода.

Приоритет имеют:

Business Criticality

Failure Impact

Complexity

Change Frequency

Security Risk

Data Risk

---

# Criticality Levels

Critical

High

Medium

Low

---

# Test Priority

```text
Critical
   ↓
High
   ↓
Medium
   ↓
Low