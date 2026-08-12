# STAIR PLATFORM

Document: 04_ENTITY_RELATIONSHIPS.md

ID: DB-0004

Status: APPROVED

---

# Purpose

Определяет правила связей между сущностями базы данных.

---

# Relationship Types

One-to-One

One-to-Many

Many-to-Many

Composition

Aggregation

Reference

---

# Allowed Relations

Project → Assembly

Assembly → Part

Part → Feature

Feature → Geometry

Geometry → Parameter

Part → Material

Assembly → Revision

Project → Documents

---

# Forbidden Relations

Pricing → Geometry Tables

Manufacturing → UI

Database → API

---

# Referential Integrity

Cascade запрещён по умолчанию.

Удаление выполняется через Domain Commands.

Soft Delete используется для большинства объектов.

Revision никогда не удаляется.

---

# Validation

Каждая связь проходит проверку целостности.

---

APPROVED