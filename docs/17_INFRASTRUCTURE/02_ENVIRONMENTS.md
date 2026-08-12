
---

# `19_INFRASTRUCTURE/02_ENVIRONMENTS.md`

```markdown
# STAIR PLATFORM

Document: 02_ENVIRONMENTS.md

ID: INFRA-0002

Status: APPROVED

---

# Purpose

Определяет environment model.

---

# Environments

## Development

Для локальной разработки.

Characteristics:

Fast Feedback

Debugging

Local Services

Synthetic Data

---

## Testing

Для automated tests.

Characteristics:

Isolated

Reproducible

Ephemeral where practical

---

## Staging

Production-like environment.

Используется для:

Integration Testing

Release Validation

Performance Testing

Security Testing

---

## Production

Рабочая среда пользователей.

---

## Recovery

Используется для Disaster Recovery и восстановления сервисов.

---

# Environment Isolation

Каждый environment имеет отдельные:

Credentials

Database

Storage

Queues

Configuration

Secrets

---

# Rules

Production credentials запрещены в Development и Testing.

---

# Configuration

Environment-specific configuration не должна изменять application code.

---

# Acceptance Criteria

Deployment в один environment не должен непреднамеренно изменять другой environment.

---

APPROVED