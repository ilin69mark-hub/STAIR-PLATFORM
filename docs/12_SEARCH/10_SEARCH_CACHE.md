# STAIR PLATFORM

Document: 10_SEARCH_CACHE.md

ID: SRCH-0010

Status: APPROVED

---

# Purpose

Определяет стратегию кэширования Search Layer.

Search Cache предназначен для снижения задержек выполнения повторяющихся запросов и уменьшения нагрузки на индекс.

---

# Cache Levels

L1 Memory Cache

L2 Redis Cache

L3 Distributed Cache

---

# Cached Objects

Search Results

Query Plans

Ranking Results

Facets

Autocomplete

Suggestions

Aggregations

---

# Cache Keys

Normalized Query

Filters

Project

Revision

User Scope

Language

---

# Invalidation

Index Updated

Object Changed

Revision Created

Permissions Changed

TTL Expired

Manual Flush

---

# Rules

Кэш не является источником истины.

После инвалидирования выполняется повторное построение результата.

---

# Acceptance Criteria

Удаление кэша не влияет на корректность поиска.

---

APPROVED