# STAIR PLATFORM

Document: 15_DATABASE_SECURITY.md

ID: SEC-0015

Status: APPROVED

---

# Purpose

Определяет Security Model Database Layer.

---

# Security Layers

```text
Application Authorization
        ↓
Repository Access
        ↓
Database Authentication
        ↓
Database Authorization
        ↓
Row-Level Security where required
        ↓
Data