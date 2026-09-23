# STAIR PLATFORM

Document: 17_AI_CONFIGURATION.md

ID: AI-0017

Status: APPROVED

---

# Purpose

Определяет конфигурацию AI Layer.

---

# Configurable Components

Runtime

Providers

Models

Prompt Templates

Context Limits

Memory

Planning

Tool Calling

RAG

Monitoring

---

# Environment Variables (S-135)

| Переменная | Значение по умолчанию | Компонент | Описание |
|---|---|---|---|
| `STAIR_AI_BASE_URL` | `https://api.openai.com/v1` | Providers | Базовый URL OpenAI-совместимого API (может указывать на OpenRouter/локальный vLLM) |
| `STAIR_AI_MODEL` | — | Models | Основная модель (chat/completions) |
| `STAIR_AI_API_KEY` | — | Providers | Ключ API (12-factor), не имеет значения по умолчанию |
| `STAIR_AI_EMBED_BASE_URL` | — | RAG | Базовый URL `/embeddings`; задан → векторный путь RAG |
| `STAIR_AI_EMBED_MODEL` | — | RAG | Модель эмбеддинга (обязательна при EMBED_BASE_URL) |
| `STAIR_AI_EMBED_API_KEY` | — | RAG | Ключ API эмбеддера |
| `STAIR_AI_RAG_TOP_K` | `5` | RAG | Число чанков на запрос (clamp 1..50) |
| `STAIR_AI_CORPUS_DIR` | `docs/13_AI,docs/ADR,docs/EDR` | RAG | Каталоги корпуса для `cmd/ai-backfill` |
| `STAIR_AI_CORPUS_VERSION` | `dev` | RAG | Версия корпуса в метаданных чанков |
| `STAIR_AI_MEMORY_TTL` | `720h` | Memory | Срок жизни сообщений conversation-memory (30 суток) |

Правила:

Все параметры проходят Validation.

Конфигурация имеет версионность.

Изменения журналируются.

---

# Configuration Sources

Configuration Files

Environment Variables

Administrative API

---

# Rules

Все параметры проходят Validation.

Конфигурация имеет версионность.

Изменения журналируются.

---

# Acceptance Criteria

AI полностью управляется конфигурацией.

---

APPROVED