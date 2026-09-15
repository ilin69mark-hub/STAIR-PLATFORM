-- STAIR PLATFORM — Demo seed data rollback (make seed down).
-- Удаляем только записи, созданные сидом (по фиксированным UUID).

DELETE FROM stair_configurations WHERE id = '00000000-0000-0000-0000-000000000301';
DELETE FROM project_members
WHERE project_id = '00000000-0000-0000-0000-000000000201';
DELETE FROM projects WHERE id = '00000000-0000-0000-0000-000000000201';
DELETE FROM users WHERE id IN (
    '00000000-0000-0000-0000-000000000101',
    '00000000-0000-0000-0000-000000000102'
);
-- Демо-tenant не удаляем: с ним связаны пользователи/проекты при первичном
-- развёртывании (дефолтный tenant создаётся миграцией 000003).