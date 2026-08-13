DROP TRIGGER IF EXISTS trg_stair_configurations_touch ON stair_configurations;
DROP TRIGGER IF EXISTS trg_projects_touch ON projects;
DROP FUNCTION IF EXISTS touch_updated_at();
DROP TABLE IF EXISTS calculations;
DROP TABLE IF EXISTS stair_configurations;
DROP TABLE IF EXISTS projects;
DROP EXTENSION IF EXISTS "pgcrypto";
