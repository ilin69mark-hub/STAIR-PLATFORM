
---

# `19_INFRASTRUCTURE/24_AUTOSCALING.md`

```markdown
# STAIR PLATFORM

Document: 24_AUTOSCALING.md

ID: INFRA-0024

Status: APPROVED

---

# Purpose

Определяет правила автоматического масштабирования.

---

# Principle

Autoscaling применяется только там, где workload behavior предсказуем.

---

# Candidates

API

Workers

Engine Workers

AI Workers

Schedulers where applicable

---

# Scaling Signals

CPU

Memory

Request Rate

Queue Depth

Job Latency

Concurrent Jobs

---

# Scaling Flow

```text
Metric
  ↓
Threshold
  ↓
Scaling Decision
  ↓
Add / Remove Instances
  ↓
Health Check