# STAIR PLATFORM

Document: 02_DATABASE_MODEL.md

ID: DB-0002

Status: APPROVED

---

# Purpose

Определяет концептуальную модель хранения инженерных данных.

---

# Core Objects

Project

Assembly

Part

Feature

Sketch

Geometry

Parameter

Constraint

Material

Revision

Manufacturing

Quotation

Document

Installation

Maintenance

---

# Relationships

Project

↓

Assembly

↓

Part

↓

Geometry

↓

Manufacturing

↓

Pricing

↓

Documents

---

# Persistence Rules

Каждый Aggregate хранится отдельно.

Каждая Revision неизменяема.

Все события журналируются.

Все зависимости индексируются.

---

# Primary Storage

Relational Data

Graph References

JSON Metadata

Binary References

Event Streams

---

APPROVED