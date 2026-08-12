# STAIR PLATFORM

Document: 14_PRICE_EVENTS.md

ID: PRC-0015

Status: APPROVED

---

# Purpose

Price Events определяет события Pricing Platform.

---

# Event Categories

Calculation Events

Validation Events

Commercial Events

Currency Events

Reporting Events

---

# Events

PricingStarted

PricingCompleted

CostCalculated

MarginApplied

DiscountApplied

TaxCalculated

CurrencyConverted

ValidationPassed

ValidationFailed

CommercialOfferGenerated

PriceUpdated

PricingFailed

---

# Event Structure

Event ID

Timestamp

Project ID

Revision

Source Engine

Correlation ID

Payload

---

# Rules

Все события:

- immutable;
- versioned;
- traceable;
- replayable.

---

# Integration

Graph Events

Manufacturing Events

API

AI

Audit

---

# Acceptance Criteria

- публикация событий после каждого этапа;
- совместимость с Event Bus;
- полная трассируемость.

---

APPROVED