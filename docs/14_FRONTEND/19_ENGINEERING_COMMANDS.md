# STAIR PLATFORM

Document: 19_ENGINEERING_COMMANDS.md

ID: FE-0019

Status: APPROVED

---

# Purpose

Определяет систему инженерных команд Frontend.

---

# Command Sources

Toolbar

Keyboard

Context Menu

Command Palette

AI

API

---

# Command Structure

Command ID

Name

Description

Parameters

Permissions

Shortcut

Handler

---

# Command Lifecycle

Register

↓

Validate

↓

Execute

↓

Update State

↓

Emit Event

---

# Command Categories

Project

Geometry

Sketch

Constraint

Graph

Manufacturing

Pricing

AI

View

---

# Keyboard Support

Commands могут иметь глобальные или контекстные shortcuts.

---

# Rules

Команды не содержат бизнес-логику.

Command Handler вызывает соответствующий Application/API service.

---

# Acceptance Criteria

Любое действие пользователя представляется единой системой команд.

---

APPROVED