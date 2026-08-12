
---

# `18_SECURITY/03_IDENTITY_AND_AUTHENTICATION.md`

```markdown
# STAIR PLATFORM

Document: 03_IDENTITY_AND_AUTHENTICATION.md

ID: SEC-0003

Status: APPROVED

---

# Purpose

Определяет Identity и Authentication Model.

---

# Identity

Каждый субъект системы имеет уникальный Identity.

---

# Identity Types

Human User

Service Account

Worker

Engine Service

AI Service

Integration

Administrator

---

# Authentication Methods

Password Authentication

Session Authentication

Token Authentication

OAuth / OIDC

Service Credentials

API Keys

---

# Authentication Flow

```text
Credentials
 ↓
Authentication Provider
 ↓
Identity Verification
 ↓
Session / Token
 ↓
Security Context