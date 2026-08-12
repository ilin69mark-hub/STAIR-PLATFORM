# STAIR PLATFORM

Document: 21_DTO_AND_MAPPING.md

ID: BE-0021

Status: APPROVED

---

# Purpose

Определяет правила DTO и Mapping.

DTO является контрактом передачи данных между архитектурными границами.

---

# DTO Types

Request DTO

Command DTO

Query DTO

Response DTO

Event DTO

Integration DTO

---

# Mapping

```text
API DTO
 ↓
Application Command
 ↓
Domain Model
```

Обратное направление:

```text
Domain Result
 ↓
Application Result
 ↓
Response DTO
```

---

# Rules

Domain Entity не возвращается непосредственно через API.

Database Model не является API DTO.

External Provider DTO не является Domain Model.

---

# Versioning

Изменение публичного DTO требует проверки API compatibility.

---

# Acceptance Criteria

Каждая внешняя граница имеет явно определенный Data Contract.

---

APPROVED