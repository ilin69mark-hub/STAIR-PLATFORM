# STAIR PLATFORM

Document: 03_COMPUTE.md

ID: INFRA-0003

Status: APPROVED

---

# Purpose

Определяет Compute Architecture.

---

# Workload Types

API

Worker

Scheduler

Engine

AI

Migration

Maintenance

---

# Compute Model

Основной deployment unit:

Container

---

# Stateless Services

API services должны быть stateless насколько это возможно.

Session state хранится во внешнем managed/shared storage.

---

# Stateful Services

Stateful workloads:

PostgreSQL

Redis

Queue

Object Storage

---

# Resource Classes

Каждый workload имеет:

CPU Limit

Memory Limit

Concurrency Limit

Timeout

Scaling Policy

---

# Resource Isolation

CPU-intensive Engine workloads не должны неконтролируемо вытеснять API workloads.

---

# Autoscaling

Autoscaling применяется к workloads с предсказуемой нагрузкой и измеряемыми scaling metrics.

---

# Scheduling

Background workloads могут использовать отдельные worker pools.

---

# Acceptance Criteria

Compute resources распределяются предсказуемо между workload types.

---

APPROVED