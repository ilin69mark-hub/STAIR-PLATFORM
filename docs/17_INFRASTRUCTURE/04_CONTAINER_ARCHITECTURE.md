# STAIR PLATFORM

Document: 04_CONTAINER_ARCHITECTURE.md

ID: INFRA-0004

Status: APPROVED

---

# Purpose

Определяет правила контейнеризации STAIR PLATFORM.

---

# Container Principle

Каждый deployable workload должен иметь определенный container boundary.

---

# Container Types

API Container

Worker Container

Engine Container

AI Container

Scheduler Container

Migration Container

---

# Image Requirements

Container Image должна:

- иметь reproducible build;
- использовать доверенный base image;
- иметь минимальный runtime footprint;
- проходить vulnerability scanning.

---

# Runtime

Production containers не должны содержать development tooling без необходимости.

---

# Configuration

Configuration передается runtime configuration mechanisms.

Secrets не встраиваются в image.

---

# Filesystem

Container filesystem должен использовать least privilege.

Read-only filesystem применяется там, где возможно.

---

# Health

Каждый long-running container должен предоставлять:

Liveness

Readiness

Startup

---

# Shutdown

Container должен корректно обрабатывать termination signal.

---

# Acceptance Criteria

Один container может быть воспроизводимо собран, запущен, проверен и остановлен.

---

APPROVED