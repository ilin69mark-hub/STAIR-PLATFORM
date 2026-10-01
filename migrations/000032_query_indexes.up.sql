-- MIG-001/002 (forensic 2026-09-26): индексы под реальные запросы.
--
-- Найдено сверкой SQL в коде с индексами из миграций 000001–000031.

-- 1) audit_events: retention-удаление.
--    AuditRepository.DeleteBefore выполняет
--      DELETE FROM audit_events WHERE created_at < $1
--    Индексы были только (tenant_id, created_at) и (project_id, created_at)
--    (миграция 000011) — ведущая колонка не created_at, поэтому retention
--    шёл полным сканированием таблицы, которая растёт неограниченно.
CREATE INDEX IF NOT EXISTS idx_audit_events_created_at
    ON audit_events (created_at);

-- 2) audit_events: аналитика за период + срез по действию.
--    analytics_repo делает выборки вида
--      WHERE tenant_id = $1 AND created_at >= $2 … [AND action = $3]
--    без составного индекса — те же данные сканировались повторно.
CREATE INDEX IF NOT EXISTS idx_audit_events_tenant_action_created
    ON audit_events (tenant_id, action, created_at DESC);

-- 3) ai_corpus_chunks: фильтр по tenant.
--    ai_repo ищет `WHERE tenant_id = $1 OR tenant_id IS NULL` (частный корпус
--    плюс общий) и `WHERE tenant_id IS NULL` (общий корпус). Индекса по
--    tenant_id не было вообще, поэтому каждый запрос ассистента сканировал
--    весь корпус.
CREATE INDEX IF NOT EXISTS idx_ai_corpus_chunks_tenant
    ON ai_corpus_chunks (tenant_id);

-- 4) project_members: lookup по пользователю (где пользователь состоит).
--    Репозиторий ищет «все проекты пользователя» — без индекса это
--    последовательное сканирование таблицы членств.
CREATE INDEX IF NOT EXISTS idx_project_members_user
    ON project_members (user_id, project_id);

-- Индексы создаются обычным CREATE INDEX IF NOT EXISTS, а не через
-- миграции с ACCESS EXCLUSIVE: CREATE INDEX (без CONCURRENTLY) берёт
-- SHARE-блокировку, которая не мешает INSERT/UPDATE/DELETE — в отличие от
-- ADD CONSTRAINT в 000031, который нужен только для первой валидации данных.
