# STAIR PLATFORM

Document: 10_IDENTIFIER_MODEL.md

ID: EDM-0010

Status: APPROVED

---

# Purpose

Определяет систему идентификации инженерных объектов.

---

# Identifier Types

UUID

Revision ID

Project Number

Drawing Number

Part Number

Assembly Number

Manufacturing ID

Installation ID

Maintenance ID

AI Session ID

Graph Node ID

---

# Principles

Все идентификаторы уникальны.

Идентификаторы неизменяемы.

Внешние номера отделены от внутренних UUID.

---

# Rules

UUID используется как первичный идентификатор.

Бизнес-номера допускают изменение.

Внешние системы используют собственные Reference ID.

---

# Acceptance Criteria

Каждый объект имеет уникальный идентификатор.