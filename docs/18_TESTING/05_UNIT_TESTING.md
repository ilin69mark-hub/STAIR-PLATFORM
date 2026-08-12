# STAIR PLATFORM

Document: 05_UNIT_TESTING.md

ID: TEST-0005

Status: APPROVED

---

# Purpose

Определяет правила Unit Testing.

---

# Scope

Unit Tests применяются к isolated components.

Основные targets:

Domain Rules

Value Objects

Entities

Specifications

Policies

Calculators

Algorithms

Validators

Pricing Rules

Geometry Algorithms

---

# Isolation

Unit Test не должен зависеть от:

Production Database

External API

Real Object Storage

External AI Provider

Network

---

# Determinism

Один и тот же test input должен приводить к одному результату.

---

# Test Structure

```text
Arrange
   ↓
Act
   ↓
Assert