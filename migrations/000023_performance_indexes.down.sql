-- Drop performance indexes
DROP INDEX IF EXISTS idx_projects_status;
DROP INDEX IF EXISTS idx_projects_created_at;

DROP INDEX IF EXISTS idx_project_members_user_id;

DROP INDEX IF EXISTS idx_sessions_token_hash;

DROP INDEX IF EXISTS idx_api_keys_token_hash;

DROP INDEX IF EXISTS idx_project_comments_created_at;

DROP INDEX IF EXISTS idx_project_reviews_decision;

DROP INDEX IF EXISTS idx_calculations_created_at;

DROP INDEX IF EXISTS idx_audit_events_action;
DROP INDEX IF EXISTS idx_audit_events_actor_id;

DROP INDEX IF EXISTS idx_orders_created_at;
