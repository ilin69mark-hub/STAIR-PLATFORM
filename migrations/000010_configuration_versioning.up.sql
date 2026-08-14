-- STAIR PLATFORM — Configuration versioning (EDR-0012, Phase C C6).
-- Revision: монотонный номер ревизии конфигурации внутри проекта (DB-0006).
-- current_configuration_id: текущая (активная) ревизия проекта.

ALTER TABLE stair_configurations
    ADD COLUMN revision INTEGER NOT NULL DEFAULT 1;

CREATE UNIQUE INDEX stair_configurations_project_revision_idx
    ON stair_configurations (project_id, revision);

ALTER TABLE projects
    ADD COLUMN current_configuration_id UUID
        REFERENCES stair_configurations(id) ON DELETE SET NULL;