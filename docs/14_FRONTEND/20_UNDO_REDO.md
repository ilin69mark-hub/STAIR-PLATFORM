# STAIR PLATFORM

Document: 20_UNDO_REDO.md

ID: FE-0020

Status: APPROVED

---

# Purpose

Определяет механизм Undo/Redo во Frontend.

---

# Objectives

- отмена пользовательских действий;
- повтор отмененных действий;
- восстановление состояния;
- интеграция с History Engine.

---

# Architecture

Command

↓

Execution

↓

History Entry

↓

Undo Stack

↓

Redo Stack

---

# Supported Operations

Create

Update

Delete

Transform

Constraint

Configuration

View

---

# Rules

Undo/Redo работает через Command System.

UI не изменяет историю напрямую.

История проекта и история UI разделены.

---

# State

Undo Stack

Redo Stack

Current Revision

Command ID

Timestamp

---

# Persistence

Project History хранится на сервере.

UI History может храниться локально.

---

# Acceptance Criteria

Пользователь может отменить и повторить поддерживаемые операции.

---

APPROVED