# STAIR PLATFORM

Document: 21_KEYBOARD_SHORTCUTS.md

ID: FE-0021

Status: APPROVED

---

# Purpose

Определяет систему клавиатурных сокращений.

---

# Shortcut Categories

Global

Workspace

Canvas

Geometry

Graph

Manufacturing

Navigation

AI

---

# Examples

Undo

Redo

Save

Search

Command Palette

Fit View

Delete

Escape

---

# Shortcut Resolution

Keyboard Event

↓

Context Detection

↓

Shortcut Registry

↓

Command

---

# Rules

Shortcut вызывает Command.

Одинаковые комбинации не должны иметь конфликтующих действий в одном Context.

Пользовательские shortcuts могут переопределять стандартные.

---

# Acceptance Criteria

Все действия, доступные через shortcuts, используют Command System.

---

APPROVED