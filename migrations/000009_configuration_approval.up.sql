-- STAIR PLATFORM — Phase C (C5): Approval.
-- EDR-0011: утверждение итоговой конфигурации владельцем проекта.
--
-- Владелец (owner) явно утверждает ревизию stair_configurations; одна
-- ревизия утверждается не более одного раза (UNIQUE). Аудит утверждений
-- проекта. Скоуп по tenant через проект (SEC-0005).

CREATE TABLE configuration_approvals (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id       UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    configuration_id UUID NOT NULL REFERENCES stair_configurations(id) ON DELETE CASCADE,
    approved_by      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    comment          TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (configuration_id)
);

CREATE INDEX configuration_approvals_project_idx ON configuration_approvals (project_id, created_at);