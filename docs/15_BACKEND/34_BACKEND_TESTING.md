# STAIR PLATFORM

Document: 34_BACKEND_TESTING.md

ID: BE-0034

Status: APPROVED

---

# Purpose

Определяет стратегию тестирования Backend.

---

# Testing Levels

Unit

Component

Integration

Contract

End-to-End

Load

Performance

Security

---

# Unit Tests

Проверяются:

Domain Rules

Application Services

Validators

Mappers

Policies

Utilities

---

# Integration Tests

Проверяются:

Database

Repositories

Cache

Queue

Storage

External Adapters

---

# Contract Tests

Проверяются контракты между:

Frontend ↔ API

Backend ↔ Engine

Backend ↔ AI

Backend ↔ External Providers

---

# E2E Tests

Основные сценарии:

Authentication

Project Creation

Geometry Modification

Constraint Processing

Calculation

Manufacturing

Pricing

Document Generation

AI Command

---

# Load Tests

Проверяются:

API Throughput

Concurrent Users

Job Queue

Worker Scaling

Database Load

Cache Performance

---

# Failure Tests

Обязательно тестируются:

Database Failure

Cache Failure

Queue Failure

Engine Failure

Provider Timeout

Worker Crash

Network Failure

---

# Acceptance Criteria

Критические Backend workflows имеют автоматизированное тестовое покрытие.

---

APPROVED