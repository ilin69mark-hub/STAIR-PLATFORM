-- STAIR PLATFORM — Configuration versioning (EDR-0012) rollback.

ALTER TABLE projects
    DROP COLUMN IF EXISTS current_configuration_id;

DROP INDEX IF EXISTS stair_configurations_project_revision_idx;

ALTER TABLE stair_configurations
    DROP COLUMN IF EXISTS revision;