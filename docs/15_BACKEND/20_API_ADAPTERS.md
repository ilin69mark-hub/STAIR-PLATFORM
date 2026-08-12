# STAIR PLATFORM

Document: 20_API_ADAPTERS.md

ID: BE-0020

Status: APPROVED

---

# Purpose

Определяет API Adapter Layer Backend.

API Adapter преобразует внешний API request в Application Command или Query.

---

# Responsibilities

Request Parsing

Authentication Context

Request Validation

DTO Mapping

Command / Query Creation

Response Mapping

Error Mapping

---

# Architecture

```text
HTTP / WebSocket
       ↓
API Adapter
       ↓
Command / Query
       ↓
Application
```

---

# Rules

API Adapter не содержит Domain Logic.

API Adapter не обращается напрямую к Repository.

API Adapter не вызывает Engine напрямую.

---

# Supported Interfaces

REST

GraphQL

WebSocket

Internal API

Public API

Partner API

---

# Acceptance Criteria

API transport может изменяться без изменения Application Layer.

---

APPROVED