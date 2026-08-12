# STAIR PLATFORM

Document: 03_ENGINE_RUNTIME.md

ID: ENG-0004

Status: APPROVED

---

# Runtime

Runtime управляет жизненным циклом всех Engine.

---

# Responsibilities

- запуск;
- остановка;
- повторный запуск;
- очереди;
- планирование;
- мониторинг;
- отмена вычислений.

---

# States

Idle

Queued

Running

Completed

Failed

Cancelled

---

# Retry

Retry допускается только для идемпотентных операций.

---

# Timeouts

Каждый Engine определяет собственный SLA.

---

APPROVED