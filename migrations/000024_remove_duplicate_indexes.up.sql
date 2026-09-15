-- Remove duplicate indexes introduced by 000023_performance_indexes
-- DB-0009: idx_project_members_user_id duplicates project_members_user_idx (000006);
-- idx_sessions_token_hash duplicates sessions_token_hash_key (UNIQUE, 000003);
-- idx_api_keys_token_hash duplicates api_keys_token_hash_key (UNIQUE, 000012).
-- Duplicates waste storage and slow down writes.

DROP INDEX IF EXISTS idx_project_members_user_id;
DROP INDEX IF EXISTS idx_sessions_token_hash;
DROP INDEX IF EXISTS idx_api_keys_token_hash;