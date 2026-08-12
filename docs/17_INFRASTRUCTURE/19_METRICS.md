# `19_INFRASTRUCTURE/19_METRICS.md`

```markdown
# STAIR PLATFORM

Document: 19_METRICS.md

ID: INFRA-0019

Status: APPROVED

---

# Purpose

Определяет Metrics Infrastructure.

---

# Metric Categories

Availability

Latency

Throughput

Errors

Resource Usage

Business Metrics

Engine Metrics

Queue Metrics

Database Metrics

---

# Golden Signals

Latency

Traffic

Errors

Saturation

---

# Application Metrics

API Request Rate

API Error Rate

API Latency

Active Jobs

Job Failure Rate

Engine Execution Time

AI Request Rate

---

# Infrastructure Metrics

CPU

Memory

Disk

Network

Container Health

Database Connections

Cache Memory

Queue Depth

---

# Engine Metrics

Geometry Operations

Constraint Solver Duration

Graph Operations

Optimization Duration

Manufacturing Jobs

---

# Cardinality

Metric labels должны иметь контролируемую cardinality.

Не следует использовать:

User ID

Request ID

Project ID

или другие high-cardinality identifiers как обычные metric labels.

---

# Alerts

Critical metrics могут быть связаны с Alert Rules.

---

# Acceptance Criteria

Основные SLO/SLA indicators могут быть измерены автоматически.

---

APPROVED
``` id="7l8yhl"

---