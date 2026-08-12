# STAIR PLATFORM

Document: 09_EVENT_PROCESSING.md

ID: BE-0009

Status: APPROVED

---

# Purpose

Определяет обработку событий Backend.

---

# Event Sources

Domain

Engine

Database

External Systems

User Actions

AI

---

# Event Pipeline

```text
Event
 ↓
Event Bus
 ↓
Event Handler
 ↓
Application Action
 ↓
State Change
```

---

# Event Types

ProjectCreated

ProjectUpdated

GeometryChanged

ConstraintChanged

GraphRebuilt

ManufacturingUpdated

PriceCalculated

DocumentGenerated

AIExecutionCompleted

---

# Event Metadata

Event ID

Event Type

Aggregate ID

Project ID

Correlation ID

Causation ID

Timestamp

Version

---

# Reliability

At Least Once Delivery

Idempotent Handlers

Retry

Dead Letter Queue

---

# Ordering

Ordering гарантируется только там, где она необходима для корректности конкретного Aggregate.

---

# Rules

Event Handler не должен предполагать, что событие будет доставлено ровно один раз.

---

# Acceptance Criteria

Повторная доставка события не приводит к некорректному состоянию системы.

---

APPROVED