# STAIR PLATFORM

Document: 05_CONTEXT_ENGINE.md

ID: AI-0005

Status: APPROVED

---

# Purpose

Определяет механизм построения контекста для AI.

Context Engine собирает, нормализует и объединяет данные из различных слоев платформы для формирования полного инженерного контекста.

---

# Objectives

- минимизация объема Prompt;
- максимальная релевантность;
- детерминированная сборка контекста;
- поддержка RAG;
- поддержка Tool Calling.

---

# Context Sources

Project

User

Domain

Graph

Geometry

Search

Storage

Manufacturing

Pricing

Installation

Maintenance

Knowledge Base

---

# Context Layers

User Context

↓

Project Context

↓

Engineering Context

↓

Geometry Context

↓

Graph Context

↓

Business Context

↓

Task Context

↓

Retrieved Knowledge

---

# Context Rules

Контекст собирается автоматически.

Источник истины не изменяется.

Все источники имеют приоритет.

Размер контекста контролируется Runtime.

---

# Acceptance Criteria

Все AI-запросы используют единый Context Engine.

---

APPROVED