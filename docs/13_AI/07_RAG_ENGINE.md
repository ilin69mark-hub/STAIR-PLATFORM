# STAIR PLATFORM

Document: 07_RAG_ENGINE.md

ID: AI-0007

Status: APPROVED

---

# Purpose

Определяет механизм Retrieval Augmented Generation.

---

# Objectives

Получение актуальных знаний.

Минимизация галлюцинаций.

Работа с инженерной документацией.

Использование Search Layer.

---

# Knowledge Sources

Domain

Database

Storage

Search

Graph

Geometry

Documentation

Standards

ADR

Knowledge Base

---

# RAG Pipeline

User Request

↓

Query Builder

↓

Search

↓

Ranking

↓

Context Selection

↓

Prompt Builder

↓

AI Model

---

# Rules

RAG использует только проверенные источники.

Все документы имеют версионность.

Используются только доступные пользователю данные.

---

# Acceptance Criteria

RAG работает через Search Layer.

---

APPROVED