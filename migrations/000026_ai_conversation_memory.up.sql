-- STAIR PLATFORM — Conversation memory ассистента (S-135, AI-0006).
-- Скоуп: tenant + project. PII-политика (S-127): сырые персоналки не
-- хранятся дольше STAIR_AI_MEMORY_TTL (default 30 дней) — TTL чистит
-- prune-джоба в cmd/api; в контекст модели попадают только последние
-- STAIR_AI_HISTORY_LIMIT сообщений (cap 50).

CREATE TABLE IF NOT EXISTS conversation_messages (
    id         BIGSERIAL PRIMARY KEY,
    tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    -- 'user' либо 'assistant' (детерминированные роли D1–D4).
    role       TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content    TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- История проекта: последние N сообщений (ORDER BY created_at DESC LIMIT n).
CREATE INDEX IF NOT EXISTS idx_conversation_messages_project
    ON conversation_messages (project_id, created_at DESC);

-- Периодический prune по TTL (DELETE WHERE created_at < now() - ttl).
CREATE INDEX IF NOT EXISTS idx_conversation_messages_created_at
    ON conversation_messages (created_at);