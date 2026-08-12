
---

# `20_TESTING/03_TEST_LEVELS.md`

```markdown
# STAIR PLATFORM

Document: 03_TEST_LEVELS.md

ID: TEST-0003

Status: APPROVED

---

# Purpose

Определяет уровни тестирования.

---

# Level 1 — Static Analysis

Проверяет:

Syntax

Types

Lint

Dependency Issues

Security Patterns

Architecture Rules

---

# Level 2 — Unit

Проверяет:

Functions

Classes / Components

Domain Rules

Algorithms

Value Objects

---

# Level 3 — Integration

Проверяет:

Database

Repositories

Queues

Storage

Services

Engine Components

---

# Level 4 — System

Проверяет:

Complete Backend

Engine Pipelines

Cross-module behavior

---

# Level 5 — End-to-End

Проверяет:

User Journeys

Critical Business Flows

Complete API flows

---

# Level 6 — Acceptance

Проверяет выполнение Product Requirements.

---

# Level 7 — Production Verification

Проверяет deployed system.

---

# Mapping

```text
Static
  ↓
Unit
  ↓
Integration
  ↓
System
  ↓
E2E
  ↓
Acceptance
  ↓
Production