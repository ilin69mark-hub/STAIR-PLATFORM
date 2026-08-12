# STAIR PLATFORM

Document: 14_GRAPH_QUERY.md

ID: ENG-GRAPH-0015

Status: APPROVED

---

# Purpose

Graph Query определяет единый механизм поиска, фильтрации и обхода объектов Graph Platform.

Graph Query является единственным публичным интерфейсом доступа к данным Graph Engine.

---

# Objectives

- унифицированный доступ к Graph;
- быстрый поиск Node и Edge;
- поддержка AI;
- поддержка аналитики;
- исключение прямого доступа к внутреннему хранилищу.

---

# Supported Operations

Find Node

Find Edge

Traverse

Filter

Aggregate

Project

Count

Exists

Shortest Path

Subgraph

---

# Query Types

ID Query

Type Query

Metadata Query

Dependency Query

Semantic Query

Path Query

Neighbor Query

Custom Predicate Query

---

# Filters

Node Type

Edge Type

Metadata

Revision

State

Owner

Tags

---

# Traversal Modes

Depth First Search

Breadth First Search

Topological Traversal

Reverse Traversal

Dependency Traversal

---

# Query Result

Query возвращает:

- Node Set
- Edge Set
- Ordered Path
- Graph View
- Metadata

---

# Performance Requirements

- индексированный поиск;
- ленивое выполнение;
- поддержка кэширования;
- потокобезопасность.

---

# Acceptance Criteria

- единый Query API;
- отсутствие прямого доступа к Graph Storage;
- возможность расширения пользовательскими запросами.

---

APPROVED