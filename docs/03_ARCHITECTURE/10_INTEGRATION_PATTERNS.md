# STAIR PLATFORM

Document: 10_INTEGRATION_PATTERNS.md

ID: ARCH-0011

Status: APPROVED

---

# Purpose

Документ определяет допустимые шаблоны интеграции между платформами.

---

# Objectives

- стандартизировать интеграции;
- обеспечить повторное использование решений;
- исключить произвольные способы взаимодействия.

---

# Supported Patterns

Request / Response

REST API

GraphQL

WebSocket

Publish / Subscribe

Domain Events

Event Bus

Background Jobs

Streaming

Batch Processing

---

# Enterprise Patterns

CQRS

Saga

Outbox

Inbox

Event Sourcing (optional)

Retry

Circuit Breaker

Bulkhead

Idempotency

Compensation

---

# Selection Rules

REST

- синхронные операции.

Events

- уведомления.

Background Jobs

- длительные операции.

Saga

- распределённые процессы.

Outbox

- гарантированная публикация событий.

---

# Forbidden Patterns

Shared Database

Shared Entity

Shared Repository

Hidden Integration

Point-to-Point without Contract

---

# Acceptance Criteria

- каждая интеграция использует утвержденный шаблон;
- отсутствуют нестандартизированные интеграции;
- все шаблоны документированы.

---

APPROVED