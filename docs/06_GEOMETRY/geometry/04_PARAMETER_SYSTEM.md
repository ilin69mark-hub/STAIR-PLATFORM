# STAIR PLATFORM

Document: 02_PARAMETER_SYSTEM.md

ID: ENG-GEO-0102

Status: APPROVED

---

# Purpose

Parameter System управляет всеми параметрами инженерной модели.

---

# Parameter Types

Integer

Float

Boolean

String

Enum

Length

Angle

Area

Volume

Mass

Material

Reference

Expression

Formula

---

# Parameter States

Defined

Calculated

Inherited

Locked

Derived

Invalid

---

# Rules

Каждый параметр имеет владельца.

Параметры могут зависеть друг от друга только через Graph Engine.

Поддерживаются формулы и выражения.

---

# Acceptance Criteria

- Вычисляемые параметры.
- Поддержка единиц измерения.
- Отслеживание зависимостей.

APPROVED