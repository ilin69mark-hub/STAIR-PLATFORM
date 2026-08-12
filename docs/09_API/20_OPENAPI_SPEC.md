# STAIR PLATFORM

Document: 20_OPENAPI_SPEC.md

ID: API-0021

Status: APPROVED

---

# Purpose

OpenAPI Specification определяет правила разработки, публикации и сопровождения контрактов REST API.

---

# Specification Standard

OpenAPI 3.1

JSON Schema

YAML

Contract First

---

# Rules

Каждый REST Endpoint обязан иметь:

- OpenAPI Specification;
- Request Schema;
- Response Schema;
- Error Schema;
- Examples;
- Security Definition;
- Version.

---

# Generation

OpenAPI является первичным источником.

Из спецификации могут генерироваться:

- Server Stubs;
- Client SDK;
- DTO;
- Validation;
- Documentation;
- Mock Server.

---

# Integration

CI/CD

API Gateway

SDK Generator

Developer Portal

Testing Platform

---

# Acceptance Criteria

- 100% REST API покрыто OpenAPI;
- отсутствие ручной синхронизации;
- автоматическая генерация артефактов.

---

APPROVED