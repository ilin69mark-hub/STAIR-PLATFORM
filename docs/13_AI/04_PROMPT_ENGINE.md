# STAIR PLATFORM

Document: 04_PROMPT_ENGINE.md

ID: AI-0004

Status: APPROVED

---

# Purpose

Определяет механизм построения AI Prompt.

---

# Prompt Components

System Prompt

Project Context

Engineering Context

Graph Context

Geometry Context

User Context

Task Context

Memory Context

---

# Prompt Pipeline

Receive Task

↓

Collect Context

↓

Build Prompt

↓

Validate

↓

Optimize

↓

Send To Model

---

# Rules

Prompt строится автоматически.

Контекст собирается из сервисов платформы.

Prompt не содержит запрещенных данных.

---

# Acceptance Criteria

Все AI-запросы проходят через Prompt Engine.

---

APPROVED