-- STAIR PLATFORM — RAG-корпус платформы (S-135, AI-0007).
-- Чанки markdown-доков (docs/13_AI, ADR, EDR, инженерные доки) с эмбеддингами
-- pgvector и FTS-fallback (tsvector/GIN).
--
-- NOTE (решение S-135): официальный образ postgres:16 НЕ содержит pgvector
-- из коробки. Расширение создаётся условно (DO-блок): если pg_available_extensions
-- не содержит `vector` (и оно не установлено), миграция продолжается без колонки
-- embedding — retriever работает по content_tsv (FTS). Это сохраняет прогон
-- тестов на штатном postgres:16 (stair-test-pg/CI) и даёт полный векторный путь
-- там, где pgvector есть (прод-RDS, локальный гейт на pgvector/pgvector:pg16).
-- Схема проектируется под embedding vector(1536) — модель эмбеддера S-135.

-- 1. Включаем pgvector, если доступен в этом окружении.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector')
       OR EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = 'vector') THEN
        CREATE EXTENSION IF NOT EXISTS vector;
    END IF;
END $$;

-- 2. Базовая таблица чанков (общая для vector и FTS путей).
CREATE TABLE IF NOT EXISTS ai_corpus_chunks (
    id          BIGSERIAL PRIMARY KEY,
    -- sha256(content) — детерминированный ключ идемпотентного backfill.
    chunk_hash  TEXT NOT NULL,
    -- Путь к исходному документу внутри репозитория (docs/13_AI/...).
    source_path TEXT NOT NULL,
    -- Версия корпуса (git ref/дата загрузки; STAIR_AI_CORPUS_VERSION).
    version     TEXT NOT NULL DEFAULT 'dev',
    doc_title   TEXT NOT NULL DEFAULT '',
    -- NULL = публичный корпус (tenant-фильтр обязателен в коде даже сейчас).
    tenant_id   UUID,
    chunk_index INTEGER NOT NULL DEFAULT 0,
    content     TEXT NOT NULL,
    -- FTS-путь ретривера: GIN-индекс по русской конфигурации.
    content_tsv tsvector GENERATED ALWAYS AS (to_tsvector('russian', content)) STORED,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Единственный чанк на (хеш, tenant). COALESCE: NULL-tenant (публичный корпус)
-- не должен ломать уникальность (в Postgres NULL-ы в UNIQUE-ограничениях
-- считаются различными).
CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_corpus_chunks_hash_tenant
    ON ai_corpus_chunks (chunk_hash, COALESCE(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid));

-- Быстрый FTS: ранжирование по релевантности + tenant-фильтр.
CREATE INDEX IF NOT EXISTS idx_ai_corpus_chunks_tsv
    ON ai_corpus_chunks USING GIN (content_tsv);

-- Поиск внутри документа по порядковому номеру чанка.
CREATE INDEX IF NOT EXISTS idx_ai_corpus_chunks_source
    ON ai_corpus_chunks (source_path, chunk_index);

-- 3. Векторный путь (только если pgvector реально доступен).
-- ADD COLUMN IF NOT EXISTS делает миграцию безопасной для повторного
-- применения после доустановки расширения на новом окружении.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'vector') THEN
        ALTER TABLE ai_corpus_chunks
            ADD COLUMN IF NOT EXISTS embedding vector(1536);
        -- HNSW (cosine) для поиска по сходству; требует pgvector >= 0.5.
        EXECUTE 'CREATE INDEX IF NOT EXISTS idx_ai_corpus_chunks_embedding
                 ON ai_corpus_chunks USING hnsw (embedding vector_cosine_ops)';
    END IF;
END $$;