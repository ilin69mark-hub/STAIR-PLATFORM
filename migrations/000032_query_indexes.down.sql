-- Откат MIG-001/002: удаляем добавленные индексы.
--
-- Обратная совместимость не нарушается: после удаления запросы снова
-- возвращаются к последовательному сканированию (то есть к прежнему
-- поведению), а данные не меняются.
DROP INDEX IF EXISTS idx_project_members_user;
DROP INDEX IF EXISTS idx_ai_corpus_chunks_tenant;
DROP INDEX IF EXISTS idx_audit_events_tenant_action_created;
DROP INDEX IF EXISTS idx_audit_events_created_at;
