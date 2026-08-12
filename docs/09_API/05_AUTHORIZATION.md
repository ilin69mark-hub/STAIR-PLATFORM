# STAIR PLATFORM

Document: 05_AUTHORIZATION.md

ID: API-0006

Status: APPROVED

---

# Purpose

Authorization Platform определяет, какие действия разрешены аутентифицированному субъекту.

---

# Objectives

- централизованное управление доступом;
- поддержка RBAC и ABAC;
- поддержка мультиарендности;
- аудит решений.

---

# Authorization Models

RBAC

ABAC

Policy Based

Resource Based

Hybrid

---

# Resource Types

Project

Geometry

Manufacturing

Pricing

Document

Drawing

Report

AI Session

User

Organization

---

# Actions

Read

Create

Update

Delete

Execute

Approve

Export

Share

Manage

---

# Decision Pipeline

Authentication

↓

Policy Evaluation

↓

Permission Resolution

↓

Decision

↓

Audit

---

# Output

Allow

Deny

Conditional Access

Audit Record

---

# Acceptance Criteria

- детерминированные решения;
- аудит всех проверок;
- поддержка расширяемых политик.

---

APPROVED