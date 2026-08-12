# STAIR PLATFORM

Document: 17_API_RATE_LIMIT.md

ID: API-0018

Status: APPROVED

---

# Purpose

Rate Limit Engine защищает API Platform от перегрузки и злоупотреблений.

---

# Objectives

- защита инфраструктуры;
- обеспечение справедливого использования;
- предотвращение DoS-атак;
- управление квотами.

---

# Rate Limit Dimensions

Per User

Per Organization

Per API Key

Per IP Address

Per OAuth Client

Per Endpoint

Per Subscription Plan

---

# Algorithms

Token Bucket

Leaky Bucket

Sliding Window

Fixed Window

Hybrid

---

# Limit Types

Requests per Second

Requests per Minute

Requests per Hour

Concurrent Connections

WebSocket Sessions

AI Requests

File Uploads

---

# Response Headers

X-RateLimit-Limit

X-RateLimit-Remaining

X-RateLimit-Reset

Retry-After

---

# Acceptance Criteria

- масштабируемое ограничение запросов;
- поддержка различных политик;
- прозрачная информация о лимитах.

---

APPROVED