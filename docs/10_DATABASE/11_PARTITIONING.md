# STAIR PLATFORM

Document: 11_PARTITIONING.md

ID: DB-0011

Status: APPROVED

---

# Purpose

Определяет стратегию горизонтального и логического разделения данных.

---

# Objectives

- масштабируемость;
- снижение размера активных таблиц;
- ускорение запросов;
- ускорение резервного копирования.

---

# Partition Candidates

Audit

Event Store

History

Revision

Logs

AI Sessions

Notifications

Metrics

---

# Partition Strategy

Range Partitioning

Hash Partitioning

List Partitioning

---

# Recommended Usage

Audit

→ Month

Event Store

→ Month

Revision

→ Project

AI Sessions

→ Month

Metrics

→ Day

---

# Archive Strategy

Активные разделы

↓

Warm Storage

↓

Cold Storage

↓

Offline Archive

---

# Rules

Партиционирование прозрачно для Repository Layer.

Переезд данных между разделами выполняется автоматически.

---

# Acceptance Criteria

Все крупные таблицы поддерживают партиционирование.

---

APPROVED