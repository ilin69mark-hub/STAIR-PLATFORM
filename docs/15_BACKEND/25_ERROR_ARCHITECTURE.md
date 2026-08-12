# STAIR PLATFORM

Document: 25_ERROR_ARCHITECTURE.md

ID: BE-0025

Status: APPROVED

---

# Purpose

Определяет единую архитектуру ошибок Backend.

---

# Error Categories

Validation Error

Authentication Error

Authorization Error

Not Found

Conflict

Domain Error

Engine Error

Persistence Error

Integration Error

Infrastructure Error

Timeout

Cancellation

Internal Error

---

# Error Structure

```text
Code

Category

Message

Details

Request ID

Correlation ID

Retryable

Metadata
```

---

# Error Flow

```text
Source
 ↓
Domain / Engine / Infrastructure Error
 ↓
Application Error
 ↓
API Error Mapping
 ↓
Client Response
```

---

# Rules

Внутренние технические детали не раскрываются клиенту.

Каждая ошибка имеет стабильный machine-readable Code.

HTTP Status Code не является единственным идентификатором ошибки.

---

# Retryable Errors

Ошибка явно определяет возможность повторного выполнения.

---

# Acceptance Criteria

Одинаковые классы ошибок имеют единое поведение во всех Backend компонентах.

---

APPROVED