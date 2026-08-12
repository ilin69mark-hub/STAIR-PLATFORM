# STAIR PLATFORM

Document: 10_INDEXING.md

ID: DB-0010

Status: APPROVED

---

# Purpose

Определяет стратегию индексирования базы данных STAIR Platform.

Indexing обеспечивает быстрый поиск инженерных объектов, минимизирует время выполнения запросов и поддерживает масштабируемость платформы.

---

# Objectives

- минимизация времени поиска;
- ускорение JOIN-операций;
- оптимизация фильтрации;
- ускорение сортировки;
- поддержка Graph Engine;
- поддержка Search Engine.

---

# Index Categories

Primary Index

Unique Index

Foreign Key Index

Composite Index

Partial Index

GIN Index

GiST Index

BRIN Index

Hash Index

---

# Mandatory Indexed Fields

UUID

Revision ID

Project ID

Assembly ID

Part ID

Parent ID

Created At

Updated At

Status

Object Type

Graph Node ID

Graph Edge ID

Correlation ID

---

# Composite Indexes

(Project ID, Revision)

(Assembly ID, Parent ID)

(Part ID, Revision)

(Object Type, Status)

(Timestamp, Event Type)

---

# JSON Indexing

GIN Index используется для:

- Metadata
- Properties
- Attributes
- Parameters
- AI Metadata

---

# Full Text Search

GIN + tsvector

используется для:

Documents

Comments

Descriptions

Metadata

---

# Rules

Все Foreign Key индексируются.

Каждый Repository обязан использовать индексируемые поля.

Полное сканирование таблиц (Sequential Scan) в production не допускается без архитектурного обоснования.

---

# Monitoring

Используется:

EXPLAIN ANALYZE

pg_stat_statements

Index Usage Statistics

---

# Acceptance Criteria

Все критические запросы используют индексы.

Регулярно выполняется анализ неиспользуемых индексов.

---

APPROVED