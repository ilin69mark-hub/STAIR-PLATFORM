-- STAIR PLATFORM — Audit System (EDR-0013, Phase G G1, SEC-0013).
-- Персистентный журнал событий безопасности: append-only, скоуп по tenant.
-- actor_id nullable (системные события), project_id nullable (глобальные).

CREATE TABLE audit_events (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id      UUID REFERENCES users(id) ON DELETE SET NULL,
    tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    project_id    UUID REFERENCES projects(id) ON DELETE CASCADE,
    action        TEXT NOT NULL,
    resource_type TEXT NOT NULL DEFAULT '',
    resource_id   TEXT NOT NULL DEFAULT '',
    result        TEXT NOT NULL DEFAULT 'ok',
    detail        TEXT NOT NULL DEFAULT '',
    request_id    TEXT NOT NULL DEFAULT '',
    ip            TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX audit_events_tenant_idx ON audit_events (tenant_id, created_at);
CREATE INDEX audit_events_project_idx ON audit_events (project_id, created_at);
