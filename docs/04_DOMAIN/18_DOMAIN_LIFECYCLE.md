# STAIR PLATFORM

Document: 18_DOMAIN_LIFECYCLE.md

ID: DOM-0018

Status: APPROVED

---

# Purpose

Определяет жизненный цикл основных инженерных объектов.

---

# Lifecycle Principles

Каждый Engineering Object проходит последовательность состояний.

Переход между состояниями фиксируется как Domain Event.

---

# Project Lifecycle

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

Manufacturing

↓

Installation

↓

Maintenance

↓

Archived

---

# Revision Lifecycle

Created

↓

Working

↓

Frozen

↓

Released

↓

Archived

---

# Document Lifecycle

Draft

↓

Generated

↓

Approved

↓

Published

↓

Archived

---

# Rules

Переход в предыдущее состояние невозможен без создания новой Revision.

Все переходы журналируются.

---

# Acceptance Criteria

- все состояния определены;
- все переходы документированы;
- каждый переход порождает Domain Event.

---

APPROVED