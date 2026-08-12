# STAIR PLATFORM

Document: 09_ATTRIBUTE_MODEL.md

ID: EDM-0009

Status: APPROVED

---

# Purpose

Определяет модель атрибутов инженерных объектов.

Attribute представляет дополнительную описательную информацию, не влияющую на инженерные вычисления.

---

# Attribute Structure

Attribute

├── Name
├── Type
├── Value
├── Required
├── Editable
├── Visibility
├── Owner
└── Revision

---

# Examples

Description

Manufacturer

Supplier

Color

Comment

Customer Code

Drawing Number

ERP Code

Barcode

QRCode

---

# Rules

Attribute не влияет на вычисления.

Attribute участвует в поиске.

Attribute индексируется.

Attribute входит в Revision.

---

# Acceptance Criteria

Attribute отделён от инженерных параметров.