
---

# `18_SECURITY/02_THREAT_MODEL.md`

```markdown
# STAIR PLATFORM

Document: 02_THREAT_MODEL.md

ID: SEC-0002

Status: APPROVED

---

# Purpose

Определяет Threat Model STAIR PLATFORM.

Threat Model используется для систематического выявления и оценки security risks.

---

# Assets

User Identity

Credentials

Sessions

Tenant Data

Projects

Geometry

Engineering Data

Manufacturing Data

Pricing Data

Documents

AI Context

API Keys

Secrets

Database

Storage

Infrastructure

---

# Threat Actors

Unauthenticated User

Authenticated User

Malicious Tenant User

Compromised Account

External Attacker

Malicious Integration

Compromised Provider

Insider

Automated Bot

---

# Threat Categories

Authentication Attacks

Authorization Bypass

Data Leakage

Injection

Account Takeover

Session Abuse

API Abuse

Resource Exhaustion

Supply Chain Attack

Malicious File

Malicious AI Input

Infrastructure Compromise

---

# Trust Boundaries

```text
Internet
   │
   ▼
API Edge
   │
   ▼
Backend
   │
   ├── Domain
   ├── Engine
   ├── AI
   └── Integrations
   │
   ▼
Data Layer