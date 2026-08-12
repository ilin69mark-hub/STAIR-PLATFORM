# STAIR PLATFORM

Document: 13_EVENT_API.md

ID: API-0014

Status: APPROVED

---

# Purpose

Event API обеспечивает обмен событиями между всеми компонентами платформы.

Event API является основой Event Driven Architecture.

---

# Objectives

- слабая связанность сервисов;
- асинхронное взаимодействие;
- масштабируемость;
- воспроизводимость событий.

---

# Event Categories

Project Events

Geometry Events

Graph Events

Manufacturing Events

Pricing Events

Document Events

AI Events

System Events

Audit Events

---

# Event Structure

Event ID

Correlation ID

Causation ID

Aggregate ID

Aggregate Type

Revision

Timestamp

Source

Payload

Metadata

---

# Delivery

Publish

Subscribe

Replay

Retry

Dead Letter Queue

---

# Guarantees

Immutable Events

Ordered Delivery (where applicable)

Versioned Payloads

Idempotent Consumers

Traceability

---

# Integration

Internal API

AI API

Webhooks

WebSocket API

Audit

---

# Acceptance Criteria

- единая модель событий;
- поддержка replay;
- поддержка Event Bus.

---

APPROVED