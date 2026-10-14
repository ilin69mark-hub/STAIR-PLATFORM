-- Откат 000036: точечные выдачи прав, мягкое удаление, роль owner.
--
-- ВНИМАНИЕ: откат снимает все точечные настройки прав. После него права
-- сотрудников снова определяются ролью — то есть система вернётся к
-- поведению «админ = всё». Таблица user_grants удаляется, поэтому перед
-- откатом снимите дамп, если настройки прав были выданы вручную.

DROP INDEX IF EXISTS idx_users_tenant_active;
DROP INDEX IF EXISTS idx_users_role_owner;

DROP TABLE IF EXISTS user_grants;

ALTER TABLE users DROP COLUMN IF EXISTS deleted_by;
ALTER TABLE users DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE users DROP COLUMN IF EXISTS grants_initialized;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('user', 'admin'));