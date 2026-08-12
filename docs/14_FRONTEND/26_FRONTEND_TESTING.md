# STAIR PLATFORM

Document: 26_FRONTEND_TESTING.md

ID: FE-0026

Status: APPROVED

---

# Purpose

Определяет стратегию тестирования Frontend Layer.

---

# Testing Levels

Unit

Component

Integration

E2E

Visual

Performance

Accessibility

---

# Unit Tests

Проверяются:

State Logic

Commands

Utilities

Adapters

Validators

---

# Component Tests

Проверяются:

UI Components

Forms

Panels

Dialogs

Widgets

---

# Integration Tests

Проверяется взаимодействие:

Frontend

API Client

State

Commands

Realtime

---

# E2E Tests

Основные сценарии:

Authentication

Project Creation

Project Editing

Geometry Editing

Graph Navigation

Manufacturing

Pricing

AI Interaction

---

# Rules

Критические пользовательские сценарии покрываются E2E.

Каждый новый Command имеет тесты.

---

# Acceptance Criteria

Критические Frontend-сценарии автоматически тестируются.

---

APPROVED