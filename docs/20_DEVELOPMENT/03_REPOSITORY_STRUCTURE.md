# STAIR PLATFORM

Document: 03_REPOSITORY_STRUCTURE.md

ID: DEV-0003

Status: APPROVED

---

# Purpose

Определяет правила организации исходного repository.

---

# Principle

Repository structure должна отражать архитектурные boundaries системы.

---

# High-Level Structure

```text
STAIR-PLATFORM/
│
├── cmd/
├── internal/
├── pkg/
├── api/
├── migrations/
├── tests/
├── scripts/
├── configs/
├── docs/
├── deployments/
└── .github/
```

---

# cmd/

Содержит application entrypoints.

Пример:

```text
cmd/
├── api/
├── worker/
└── migrate/
```

---

# internal/

Содержит private application implementation.

Пример:

```text
internal/
├── domain/
├── application/
├── engine/
├── geometry/
├── manufacturing/
├── pricing/
├── infrastructure/
└── transport/
```

---

# pkg/

Содержит reusable packages, если их публичное использование действительно необходимо.

---

# api/

Содержит API contracts и связанные specification artifacts.

---

# migrations/

Содержит database migrations.

---

# tests/

Содержит cross-component и system-level tests.

---

# scripts/

Содержит development и automation scripts.

---

# configs/

Содержит configuration templates и non-secret configuration definitions.

Secrets не хранятся в repository.

---

# docs/

Содержит техническую документацию, если она относится непосредственно к source repository.

---

# deployments/

Содержит deployment-related manifests и configuration.

---

# .github/

Содержит:

* CI/CD
* Pull Request Templates
* Issue Templates
* Repository Automation

---

# Boundary Rule

Code не должен размещаться в случайных директориях только ради удобства.

Каждый package должен иметь определенную responsibility.

---

# Acceptance Criteria

Repository structure соответствует архитектурным boundaries и позволяет однозначно определить ownership каждого компонента.

---

APPROVED
