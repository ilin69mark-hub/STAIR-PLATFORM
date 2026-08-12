
---

# `18_SECURITY/09_SECRETS_MANAGEMENT.md`

```markdown
# STAIR PLATFORM

Document: 09_SECRETS_MANAGEMENT.md

ID: SEC-0009

Status: APPROVED

---

# Purpose

Определяет управление Secrets.

---

# Secrets

Database Credentials

JWT Signing Keys

Encryption Keys

API Keys

OAuth Secrets

Provider Credentials

Storage Credentials

---

# Storage

Secrets должны храниться в:

Secret Manager

Environment Secret Store

Deployment Secret Store

или другом защищенном механизме.

---

# Forbidden

Secrets запрещено хранить:

Source Code

Git Repository

Docker Image

Public Configuration

Logs

Client Bundle

---

# Access

Secrets выдаются только компонентам, которым они необходимы.

---

# Rotation

Критические secrets должны поддерживать rotation.

---

# Revocation

При компрометации Secret должна существовать возможность:

Revoke

Rotate

Redeploy

Audit

---

# Secret Exposure

При обнаружении утечки:

```text
Detect
 ↓
Revoke
 ↓
Rotate
 ↓
Investigate
 ↓
Audit
 ↓
Recover