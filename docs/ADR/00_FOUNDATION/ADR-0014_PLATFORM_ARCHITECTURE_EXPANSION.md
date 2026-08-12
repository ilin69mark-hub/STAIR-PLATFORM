# ADR-0014

Title:
Platform Architecture Expansion After Architecture Gap Review

Status:
ACCEPTED

Date:
2026-08-04

Decision Makers:
Architecture Board

---

# Context

После завершения проектирования следующих разделов:

- ENGINE
- GEOMETRY
- MANUFACTURING
- PRICING
- API

была проведена полная Architecture Gap Review.

Критических архитектурных ошибок не выявлено.

Архитектура признана зрелой и пригодной для дальнейшего развития.

Однако анализ показал отсутствие нескольких платформенных подсистем, которые являются стандартом для современных CAD/CAM/PLM/ERP платформ Enterprise-класса.

Для предотвращения архитектурного долга принято решение расширить архитектуру платформы.

---

# Decision

Добавить новые платформенные разделы.

Все новые разделы являются независимыми Platform Modules.

Они не изменяют существующие ENGINE, GEOMETRY, MANUFACTURING, PRICING и API.

Все взаимодействие осуществляется исключительно через API Platform и Event Platform.

---

# Accepted Architecture Principles

## API First

Все платформы публикуют только API.

---

## Contract First

Все контракты определяются до реализации.

---

## Event Driven

Все значимые изменения публикуются через Event Platform.

---

## Canonical Data Model

Все платформы взаимодействуют исключительно через Canonical DTO.

Передача внутренних Entity запрещена.

---

## No Cross Database Access

Ни одна платформа не имеет права обращаться к БД другой платформы.

Взаимодействие осуществляется исключительно через API и Event Platform.

---

## Platform Independence

Каждая Platform является независимым модулем.

---

## Revision First

Все данные платформы поддерживают Revision.

---

## Immutable Audit

История изменений не удаляется.

---

## Observability First

Каждая операция поддерживает:

- Logs
- Metrics
- Tracing
- Events

---

# New Architecture Documents

Добавить в раздел:

03_ARCHITECTURE

следующие документы:

CANONICAL_DATA_MODEL.md

COMMON_CONTRACTS.md

CONFIGURATION_ARCHITECTURE.md

PLATFORM_INTERACTION_RULES.md

PLATFORM_BOUNDARIES.md

---

# New Platform Sections

Добавить следующие разделы проекта.

09_DATABASE

единая архитектура хранения данных

---

10_STORAGE

Blob Storage

Files

Versions

Attachments

Previews

Checksums

Object Storage

---

11_SEARCH

Search Engine

Indexing

Full Text Search

Geometry Search

Document Search

Semantic Search

---

12_IDENTITY

Organizations

Users

Teams

Roles

Permissions

Tenants

Invitations

Profiles

---

13_NOTIFICATION

Email

Push

Telegram

SMS

Desktop

Webhook Notifications

Notification Templates

---

14_AUDIT

Audit Events

Security Audit

Data Changes

Permission Changes

System Audit

Compliance

---

15_PLUGIN

Plugin SDK

Marketplace

Extensions

Custom Generators

Custom Tools

AI Plugins

---

16_SDK

Go SDK

TypeScript SDK

Python SDK

C# SDK

Rust SDK

OpenAPI SDK

---

17_AI

AI Runtime

AI Workers

AI Memory

Prompt Engine

Tool Calling

Context Engine

Reasoning Engine

Model Router

Embeddings

Retrieval

Knowledge Base

---

18_BACKEND

Application Services

Business Logic

Repositories

CQRS

Jobs

Background Workers

Transactions

---

19_FRONTEND

Desktop

Web

Mobile

UI Components

Canvas

Editor

State Management

---

20_INFRASTRUCTURE

Docker

Kubernetes

CI/CD

Monitoring

Logging

Tracing

Secrets

Cloud

Backup

Recovery

---

# Additional Platform Modules

Добавить следующие независимые платформенные сервисы.

Job Platform

Platform Scheduler

Feature Flags

Licensing

Configuration Service

Search Service

Audit Service

Notification Service

---

# Future Documents

После завершения основных разделов добавить:

PRICE_LISTS.md

PURCHASED_COMPONENTS.md

QUOTATION_ENGINE.md

PRICING_RULESETS.md

---

# Consequences

Положительные:

- уменьшение связанности платформ;
- отсутствие циклических зависимостей;
- готовность к микросервисной архитектуре;
- масштабируемость;
- возможность независимого развития платформ;
- единая модель интеграции;
- соответствие Enterprise Architecture.

Отрицательные:

- увеличение количества архитектурной документации;
- увеличение первоначального объема проектирования.

Данные последствия признаны допустимыми.

---

# Architecture Status

Architecture Review:

PASSED

Critical Issues:

NONE

Architecture Readiness:

APPROVED FOR DATABASE DESIGN

---

Decision

ACCEPTED