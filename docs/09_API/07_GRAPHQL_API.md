# STAIR PLATFORM

Document: 07_GRAPHQL_API.md

ID: API-0008

Status: EXPERIMENTAL (S-131: в проде НЕ выставлен, см. Implementation Status)

---

# Implementation Status (S-131)

- Пакет `internal/transport/graphql` — протестированная библиотека
  (resolver-слой: 7 query + 6 mutation + 3 subscription метода; handler +
  тесты зеленые в общем сьюте). В production-роутере НЕ зарегистрирован
  (см. коммент в `internal/transport/http/router.go`).
- HTTP-диспетчер библиотеки — наивный substring-match, обслуживает только
  5 операций (stairConfiguration, projectConfigurations,
  createStairConfiguration, runAnalysis, runPipeline); остальные возвращают
  `unknown query`. Полноценного парсера/валидации `schema.graphql` нет.
- Решение S-131 (ADR на борде): НЕ допиливать самописный движок и НЕ
  удалять протестированный код. Полная реализация — только на зрелом
  движке (gqlgen) отдельной задачей, с auth (requireAuth), depth/complexity
  лимитами и rate-limit; `schema.graphql` — контракт для этой работы.
  До тех пор док имеет статус EXPERIMENTAL, а не APPROVED.

---

# Purpose

GraphQL API предоставляет клиентам возможность получать только необходимые данные и выполнять сложные запросы через единый endpoint.

---

# Objectives

- уменьшение объема передаваемых данных;
- поддержка сложных клиентских сценариев;
- агрегирование информации из нескольких платформ.

---

# Root Types

Query

Mutation

Subscription

---

# Data Sources

Engine

Geometry

Manufacturing

Pricing

Documents

AI

User Management

---

# Schema Principles

Schema First

Strong Typing

Version Awareness

Deprecation Policy

Federation Ready

---

# Security

Authentication

Authorization

Depth Limiting

Complexity Analysis

Rate Limiting

---

# Output

Typed Response

Errors

Extensions

Metadata

---

# Acceptance Criteria (целевые — при полной реализации на движке, S-131)

- единая схема GraphQL;
- строгая типизация;
- поддержка подписок и федерации.

---

APPROVED