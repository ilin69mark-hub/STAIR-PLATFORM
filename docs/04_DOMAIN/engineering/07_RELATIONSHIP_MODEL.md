# STAIR PLATFORM

Document: 07_RELATIONSHIP_MODEL.md

ID: EDM-0007

Status: APPROVED

---

# Purpose

Определяет модель отношений между инженерными объектами.

---

# Relationship Types

Parent

Child

Reference

Dependency

Association

Ownership

Containment

Composition

Aggregation

---

# Engineering Relationships

Project

↓

Assembly

↓

Part

↓

Feature

↓

Geometry

↓

Manufacturing

↓

Pricing

↓

Documents

---

# Graph Mapping

Каждое отношение отображается в Graph Engine.

Все зависимости имеют уникальный идентификатор.

---

# Rules

Циклические отношения запрещены, если это не предусмотрено явно типом связи.

Удаление объекта не должно оставлять "висячих" ссылок.

Все связи проходят проверку целостности.

---

# Acceptance Criteria

Все связи документированы.

Graph Engine способен восстановить полную структуру объекта.

---

APPROVED