
---

# `18_SECURITY/14_ENGINE_SECURITY.md`

```markdown
# STAIR PLATFORM

Document: 14_ENGINE_SECURITY.md

ID: SEC-0014

Status: APPROVED

---

# Purpose

Определяет Security Model Engineering Engines.

---

# Protected Engines

Geometry Engine

Graph Engine

Constraint Solver

Optimization Engine

Manufacturing Engine

Pricing Engine

---

# Principle

Engine не доверяет вызывающему компоненту автоматически.

Каждый Engine invocation должен иметь определенный execution context.

---

# Engine Context

```text
User

Tenant

Project

Operation

Permission

Correlation ID

Job ID
