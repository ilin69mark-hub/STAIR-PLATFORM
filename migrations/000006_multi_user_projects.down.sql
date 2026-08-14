-- STAIR PLATFORM — Phase C (C1): откат Multi-user Projects.

DROP INDEX IF EXISTS projects_owner_idx;
DROP INDEX IF EXISTS project_members_user_idx;
DROP INDEX IF EXISTS project_members_one_owner;
DROP TABLE IF EXISTS project_members;
ALTER TABLE projects DROP COLUMN IF EXISTS owner_id;