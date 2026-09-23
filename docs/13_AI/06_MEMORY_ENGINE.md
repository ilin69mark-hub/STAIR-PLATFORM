# STAIR PLATFORM

Document: 06_MEMORY_ENGINE.md

ID: AI-0006

Status: APPROVED

---

# Purpose

Определяет механизм памяти AI.

Memory Engine хранит рабочий контекст взаимодействия AI без нарушения архитектурных границ платформы.

---

# Implementation (S-135)

Действующая реализация Conversation Memory (S-135, AI-0006):

- **Хранилище**: таблица `conversation_messages` (миграция 000026),
  `id BIGSERIAL` + `(tenant_id, project_id, role, content, created_at)`;
  FK `tenant_id/project_id` с `ON DELETE CASCADE`.
- **Скоуп записи**: `(tenant_id, project_id)`; скоуп требуется всегда —
  вызовы без него отклоняются хранилищем. PII-правило S-127: в сообщения
  пишется только детерминированное резюме запроса (kind/размеры/тип марша)
  и сериализованный ответ, конфигурации пользователей целиком не хранятся.
  Пустой `project_id` в запросе — память для запроса не используется вовсе.
- **TTL**: по умолчанию 30 суток (`STAIR_AI_MEMORY_TTL`); фоновый prune
  (`DELETE WHERE created_at < now() - ttl`) запускается из `cmd/api` горутиной
  при старте и далее раз в TTL (best-effort).
- **История в контекст**: `project_id`/`history_limit` в запросе;
  limit 0 → default 10, cap 50, <0 → без истории. Блок
  `История диалога по проекту (последние N)` рендерится в контекст промпта;
  в `Response.Notes` попадают только RAG-цитаты (см. AI-0007).
- **Best-effort**: сбой памяти (в т.ч. отсутствие миграции) не роняет запрос —
  RAG/память выполняются best-effort, результат возвращается без блока истории.

## Env (Memory)

| Переменная | Значение по умолчанию | Описание |
|---|---|---|
| `STAIR_AI_MEMORY_TTL` | `720h` (30 суток) | Срок жизни сообщений в памяти (duration) |

---

# Memory Types

Session Memory

Conversation Memory

Project Memory

Task Memory

Temporary Memory

Persistent Memory

---

# Memory Scope

User Session

Project

Organization

AI Worker

Workflow

---

# Memory Operations

Create

Read

Update

Expire

Archive

Delete

---

# Rules

Memory не является источником истины.

Memory может быть полностью восстановлена.

Memory имеет срок жизни.

---

# Acceptance Criteria

AI способен использовать память между последовательными действиями.

---

APPROVED