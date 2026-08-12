# STAIR PLATFORM

Document: 04_CANONICAL_DATA_MODEL.md

ID: ARCH-0005

Status: APPROVED

---

# Purpose

Canonical Data Model (CDM) определяет единый язык обмена данными между всеми платформами STAIR Platform.

CDM является обязательным контрактом для всех внутренних и внешних интеграций.

Ни одна платформа не имеет права публиковать свои внутренние Entity за пределами собственной границы.

---

# Objectives

- единая модель обмена данными;
- независимость внутренних моделей;
- совместимость платформ;
- поддержка версионирования;
- упрощение интеграции;
- поддержка AI и SDK.

---

# Architecture Principle

```
Internal Entity

↓

Mapper

↓

Canonical DTO

↓

API/Event Platform

↓

Mapper

↓

Internal Entity
```

Внутренние Entity никогда не покидают границы своей платформы.

---

# Canonical Objects

CDM определяет следующие базовые объекты:

Project

Assembly

Part

Sketch

Feature

Geometry

Material

Operation

Machine

Tool

Price

Estimate

Quotation

Document

Revision

Organization

User

Attachment

Notification

Audit Record

AI Session

AI Request

AI Response

---

# DTO Requirements

Каждый DTO обязан содержать:

Identifier

Revision

Created At

Updated At

Metadata

Owner

Version

---

# Mapping Rules

Entity → DTO

DTO → Entity

Event → DTO

API → DTO

Database → Entity

---

# Versioning

Все DTO являются версионируемыми.

Удаление полей запрещено.

Добавление новых полей допускается только при сохранении обратной совместимости.

---

# Constraints

Запрещено:

- передавать Entity;
- использовать Database Models как API Models;
- использовать разные DTO для одинаковых сущностей без ADR.

---

# Related Documents

COMMON_CONTRACTS

API

DATABASE

SDK

---

# Acceptance Criteria

- все платформы используют единые DTO;
- отсутствует передача Entity;
- все DTO поддерживают версионирование.

---

APPROVED