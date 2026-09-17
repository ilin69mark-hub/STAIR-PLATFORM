-- Restore duplicate indexes (rollback of 000024; keeps pre-/post-000023 parity)
CREATE INDEX IF NOT EXISTS idx_project_members_user_id ON project_members(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_token_hash ON api_keys(token_hash);