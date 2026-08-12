# STAIR PLATFORM

Document: 10_ENGINE_SCHEDULER.md

ID: ENG-0011

Status: APPROVED

---

# Purpose

Engine Scheduler управляет порядком выполнения вычислительных задач между Engine.

Scheduler является частью Runtime и взаимодействует с Graph Scheduler.

---

# Responsibilities

- планирование выполнения;
- управление очередями;
- распределение задач;
- управление приоритетами;
- балансировка нагрузки;
- отмена вычислений;
- повторный запуск.

---

# Scheduling Principles

Scheduler:

- deterministic;
- dependency aware;
- priority based;
- parallel ready;
- cancelable.

---

# Task Lifecycle

Created

↓

Queued

↓

Scheduled

↓

Running

↓

Completed

или

↓

Failed

или

↓

Cancelled

---

# Priorities

Critical

High

Normal

Low

Background

---

# Parallel Execution

Допускается параллельное выполнение задач при отсутствии зависимостей.

Graph Platform определяет допустимый порядок вычислений.

---

# Cancellation

Scheduler обязан поддерживать:

- отмену Pipeline;
- отмену отдельных задач;
- безопасную остановку Worker;
- очистку временных ресурсов.

---

# Retry Policy

Допускается повторное выполнение только идемпотентных операций.

Количество попыток определяется конфигурацией.

---

# Resource Management

Scheduler управляет:

- Worker Pool;
- CPU Budget;
- Memory Budget;
- Queue Length;
- Timeout.

---

# Failure Handling

При ошибке:

- публикуется Engine Event;
- освобождаются ресурсы;
- сохраняется диагностика;
- уведомляется Runtime.

---

# Acceptance Criteria

- детерминированное планирование;
- поддержка параллельного выполнения;
- безопасная отмена вычислений;
- масштабирование Worker Pool.

---

APPROVED