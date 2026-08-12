# STAIR PLATFORM

Document: 06_REST_API.md

ID: API-0007

Status: APPROVED

---

# Purpose

REST API предоставляет стандартный HTTP-интерфейс для клиентских приложений и внешних интеграций.

---

# Design Principles

Resource Oriented

Stateless

Cache Friendly

Versioned

Contract First

---

# Resource Categories

Projects

Geometry

Manufacturing

Pricing

Documents

Reports

Users

Organizations

AI

Administration

---

# HTTP Methods

GET

POST

PUT

PATCH

DELETE

---

# Response Format

Request ID

Correlation ID

Timestamp

Status

Payload

Errors

Metadata

---

# Pagination

Offset

Limit

Cursor

---

# Filtering

Field Filters

Sorting

Search

Full Text

---

# Acceptance Criteria

- единый REST-контракт;
- поддержка пагинации и фильтрации;
- обратная совместимость между версиями.

---

APPROVED