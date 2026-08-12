# STAIR PLATFORM

Document: 09_LOCAL_DEVELOPMENT_SETUP.md

ID: DEV-0009

Status: APPROVED

---

# Purpose

Определяет требования к локальной среде разработки.

---

# Objectives

Локальная среда должна быть:

* воспроизводимой
* изолированной
* быстрой
* максимально близкой к Production

---

# Required Components

* Go
* PostgreSQL
* Redis
* Docker
* Git
* Make (или эквивалентный task runner)

---

# Local Architecture

```text
Developer
    │
    ▼
Frontend
    │
    ▼
API
    │
 ┌──┼─────────┐
 ▼  ▼         ▼
DB Redis   Storage
```

---

# Configuration

Используются локальные environment configuration files.

Secrets не коммитятся в repository.

---

# Database

Локальная база создается автоматически или через воспроизводимый bootstrap process.

---

# Development Commands

Проект должен предоставлять единый набор команд для:

* setup
* run
* test
* migrate
* seed
* lint
* build

---

# Hot Reload

Допускается использование hot reload только в локальной разработке.

---

# Environment Reset

Developer должен иметь возможность полностью пересоздать локальную среду без ручных действий.

---

# Acceptance Criteria

Новый разработчик может развернуть рабочую локальную среду по воспроизводимой инструкции.

---

APPROVED
