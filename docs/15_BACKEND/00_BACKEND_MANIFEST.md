# STAIR PLATFORM

Document: 00_BACKEND_MANIFEST.md

ID: BE-0000

Status: APPROVED

---

# Purpose

Backend Layer определяет серверную часть STAIR PLATFORM.

Backend является связующим application-слоем между API, Product, Domain, Engine, Database, Storage, Search, AI и Infrastructure.

---

# Responsibilities

API Processing

Authentication

Authorization

Application Orchestration

Domain Interaction

Engine Coordination

Background Jobs

Transactions

Persistence

Events

Integration

---

# Backend Components

API Handlers

Application Services

Domain Services

Repositories

Workers

Schedulers

Event Handlers

Integration Services

---

# Architectural Position

Frontend

↓

API

↓

Backend

↓

Domain / Engine

↓

Infrastructure

---

# Principles

Domain Driven Design

Separation of Concerns

Dependency Inversion

Stateless Services

Explicit Transactions

Event Driven Processing

---

# Rules

Backend не дублирует Geometry, Graph или Manufacturing Engine.

Backend управляет application flow и orchestration.

---

# Related Documents

03_DOMAIN

04_ENGINE

08_API

10_DATABASE

11_STORAGE

12_SEARCH

13_AI

14_SECURITY

15_INFRASTRUCTURE

16_FRONTEND

---

# Acceptance Criteria

Backend имеет четкие границы ответственности и не нарушает архитектурные границы платформы.

---

APPROVED