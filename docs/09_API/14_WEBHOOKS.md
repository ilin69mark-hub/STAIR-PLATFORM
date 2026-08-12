# STAIR PLATFORM

Document: 14_WEBHOOKS.md

ID: API-0015

Status: APPROVED

---

# Purpose

Webhook Platform обеспечивает доставку событий внешним системам.

---

# Objectives

- уведомление партнеров;
- интеграция без polling;
- надежная доставка.

---

# Supported Events

Project Updated

Revision Created

Drawing Generated

Manufacturing Started

Manufacturing Finished

Pricing Updated

Commercial Offer Generated

AI Task Finished

Document Generated

User Invited

Organization Updated

---

# Delivery Policy

Retry

Exponential Backoff

Dead Letter Queue

Signature Validation

Timeout

---

# Security

HTTPS Only

HMAC Signature

Timestamp Validation

Replay Protection

IP Validation

---

# Webhook Structure

Webhook ID

Event Type

Payload

Signature

Retry Count

Delivery Status

Timestamp

---

# Acceptance Criteria

- гарантированная доставка;
- проверка подписи;
- повторная отправка при ошибках.

---

APPROVED