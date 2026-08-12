
---

# `19_INFRASTRUCTURE/22_SLO_SLA.md`

```markdown
# STAIR PLATFORM

Document: 22_SLO_SLA.md

ID: INFRA-0022

Status: APPROVED

---

# Purpose

Определяет reliability targets инфраструктуры.

---

# Definitions

SLA

Service Level Agreement

SLO

Service Level Objective

SLI

Service Level Indicator

---

# Core SLIs

Availability

Latency

Error Rate

Job Success Rate

Data Durability

Recovery Time

---

# API SLO

Основные показатели:

Availability

Request Latency

5xx Error Rate

---

# Async SLO

Job Processing Latency

Job Success Rate

Queue Delay

---

# Engine SLO

Execution Success Rate

Execution Latency

Timeout Rate

---

# Storage SLO

Availability

Durability

Upload Success Rate

Download Success Rate

---

# Database SLO

Availability

Query Latency

Connection Availability

Recovery Capability

---

# Error Budget

Для каждого критического SLO может использоваться Error Budget.

```text
SLO
 ↓
Error Budget
 ↓
Budget Remaining
 ↓
Release / Reliability Decision