# STAIR PLATFORM

Document: 13_CACHE.md

ID: BE-0013

Status: APPROVED

---

# Purpose

Определяет стратегию Backend Cache.

---

# Cache Targets

Session Data

Project Metadata

Permissions

Read Models

Graph Data

Search Results

Expensive Calculations

Temporary Job State

---

# Cache Strategies

Cache Aside

Read Through

Write Through

TTL

Explicit Invalidation

---

# Cache Key

```text
<domain>:<entity>:<id>:<version>
```

---

# Invalidation

Cache должен инвалидироваться при изменении исходных данных.

---

# Consistency

Database является Source of Truth.

Cache не является источником истины.

---

# Failure

При недоступности Cache Backend должен иметь fallback на primary storage, если это допустимо для конкретного сценария.

---

# Rules

Критические данные не должны существовать только в Cache.

---

# Acceptance Criteria

Cache повышает производительность без нарушения корректности данных.

---

APPROVED