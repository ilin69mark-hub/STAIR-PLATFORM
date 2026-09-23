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

# Implementation (S-135)

Действующая реализация RAG-корпуса (S-135, AI-0007):

- **Корпус**: markdown-доки репозитория (`docs/13_AI`, `docs/ADR`, `docs/EDR`); чанкер
  нацелен на 500–800 токенов с overlap ~100; метаданные чанка — путь, версия,
  заголовок, индекс.
- **Хранилище**: таблица `ai_corpus_chunks` (миграция 000025) с двумя путями поиска:
  - `pgvector` — колонка `embedding vector(1536)` + HNSW (косинус), когда
    расширение `vector` доступно (прод-RDS, локальный гейт `pgvector/pgvector:pg16`);
  - **FTS-fallback** — `content_tsv` tsvector/GIN (russian) на штатном `postgres:16`
    БЕЗ pgvector. Ретривер автоматически деградирует на FTS при недоступности
    расширения или сбое эмбеддера.
- **Идемпотентность**: ключ `(chunk_hash, COALESCE(tenant_id, zero-uuid))`;
  повторный backfill обновляет метаданные и не создаёт дублей; идентичные
  эмбеддинги не переписываются.
- **Tenant-фильтр (SEC-0005)**: каждый поиск ограничен `tenant_id = $1 OR
  tenant_id IS NULL` (конфигурации пользователей в корпус не складываются).
- **Интеграция**: `assistant.Service.WithRAG(retriever, topK)`; найденные чанки
  попадают в контекст модели, цитаты `[source: <путь>, §N]` — в `Response.Notes`,
  идентификаторы использованных чанков — в detail события аудита (`rag_sources=`).

## Env (RAG)

| Переменная | Значение по умолчанию | Описание |
|---|---|---|
| `STAIR_AI_EMBED_BASE_URL` | — (RAG off) | Базовый URL OpenAI-совместимого `/embeddings`; задан → векторный путь |
| `STAIR_AI_EMBED_MODEL` | — | Модель эмбеддинга (обязательна при заданном BASE_URL) |
| `STAIR_AI_EMBED_API_KEY` | — | Ключ API (env-правило 12-factor) |
| `STAIR_AI_RAG_TOP_K` | `5` | Число чанков на запрос (clamp 1..50) |
| `STAIR_AI_CORPUS_DIR` | `docs/13_AI,docs/ADR,docs/EDR` | Каталоги корпуса (backfill) |
| `STAIR_AI_CORPUS_VERSION` | `dev` | Версия корпуса в метаданных чанков |

## Runbook (backfill)

```bash
# FTS-only backfill (без эмбеддингов — работает на любом postgres:16):
STAIR_DATABASE_URL="postgres://..." go run ./cmd/ai-backfill -version v1.2.0

# Полный backfill c эмбеддингами через OpenRouter (OpenAI-совместимый /embeddings):
STAIR_DATABASE_URL="postgres://..." \
  STAIR_AI_EMBED_BASE_URL="https://openrouter.ai/api/v1" \
  STAIR_AI_EMBED_MODEL="openai/text-embedding-3-small" \
  STAIR_AI_EMBED_API_KEY="sk-or-..." \
  go run ./cmd/ai-backfill -version v1.2.0 -dry-run   # сначала посмотреть объём

# Проверка готовности векторного пути на инстансе:
#   SELECT typname FROM pg_type WHERE typname = 'vector';
```

Повторный запуск с тем же корпусом идемпотентен (число «new» в логе = 0).

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