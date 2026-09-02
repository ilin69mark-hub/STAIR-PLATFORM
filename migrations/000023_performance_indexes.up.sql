-- Performance indexes for common query patterns
-- These indexes improve query performance for multi-tenant SaaS workload
-- NOTE: IF NOT EXISTS makes this idempotent; safe to re-run.

-- Projects: status and created_at for filtered/sorted lists
CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);
CREATE INDEX IF NOT EXISTS idx_projects_created_at ON projects(created_at DESC);

-- Project members: frequent lookups by user_id (project_id already covered by PK)
CREATE INDEX IF NOT EXISTS idx_project_members_user_id ON project_members(user_id);

-- Sessions: token hash lookup for authentication (user_id and expires_at already indexed)
CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions(token_hash);

-- API keys: token hash lookup for authentication (tenant_id already indexed)
CREATE INDEX IF NOT EXISTS idx_api_keys_token_hash ON api_keys(token_hash);

-- Project comments: project_id already indexed; add created_at for sorting
CREATE INDEX IF NOT EXISTS idx_project_comments_created_at ON project_comments(created_at DESC);

-- Project reviews: project_id already indexed; add decision for filtered queries
CREATE INDEX IF NOT EXISTS idx_project_reviews_decision ON project_reviews(decision);

-- Calculations: project_id already indexed in init; add created_at for sorting
CREATE INDEX IF NOT EXISTS idx_calculations_created_at ON calculations(created_at DESC);

-- Audit events: action for filtered queries (tenant_id and created_at already indexed)
CREATE INDEX IF NOT EXISTS idx_audit_events_action ON audit_events(action);
CREATE INDEX IF NOT EXISTS idx_audit_events_actor_id ON audit_events(actor_id);

-- Orders: created_at for sorting (tenant_id and status already indexed)
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at DESC);
