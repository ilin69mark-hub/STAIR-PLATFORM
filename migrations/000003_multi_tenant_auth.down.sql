-- STAIR PLATFORM — MVP-11 (F2): откат Identity/AuthN/AuthZ/Tenant.
-- Порядок обратный созданию; проекты теряют привязку к tenant'у.

DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
ALTER TABLE projects DROP COLUMN tenant_id;
DROP TABLE IF EXISTS tenants;
