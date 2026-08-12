# STAIR PLATFORM

Document: 11_STATE_MODEL.md

ID: EDM-0011

Status: APPROVED

---

# Purpose

Определяет модель состояний инженерных объектов.

---

# Generic State

Created

↓

Draft

↓

Editing

↓

Calculated

↓

Validated

↓

Approved

↓

Released

↓

Archived

---

# State Rules

Состояние изменяется только через Domain Command.

Каждый переход создаёт Domain Event.

Состояния полностью журналируются.

---

# Acceptance Criteria

Все Engineering Objects используют единую State Model.