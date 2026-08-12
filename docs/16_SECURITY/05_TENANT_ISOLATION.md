# STAIR PLATFORM

Document: 05_TENANT_ISOLATION.md

ID: SEC-0005

Status: APPROVED

---

# Purpose

Определяет модель изоляции данных между Tenant.

---

# Principle

Tenant является базовой границей изоляции данных.

Пользователь одного Tenant не должен иметь возможность получить или изменить данные другого Tenant.

---

# Isolation Layers

```text
Authentication
      ↓
Tenant Context
      ↓
Application Authorization
      ↓
Repository Filtering
      ↓
Database Isolation