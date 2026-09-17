-- STAIR PLATFORM — Demo seed data (DEV-0009: make seed).
-- Idempotent: фиксированные UUID, ON CONFLICT DO NOTHING — повторный запуск
-- безопасен. Пароль демо-пользователей: demo1234 (bcrypt-хеш ниже).
-- Данные создаются только если их нет (на production не провалится).
--
-- NOTE: seed применяется как миграция из каталога migrations/seeds/ в
-- отдельную таблицу версий (schema_migrations_seeds) и не конфликтует с
-- основными миграциями.

-- Демо-пользователи привязываются к дефолтному tenant'у (миграция 000003).
-- Bootstrap-админ (admin@admin.ru / demo1234) нужен для админ-панели на свежем окружении.
INSERT INTO users (id, tenant_id, email, name, password_hash, role, status)
SELECT
    d.id, t.id, d.email, d.name, d.password_hash, d.role, d.status
FROM (VALUES
    ('00000000-0000-0000-0000-000000000101'::uuid,
     'owner@demo.local', 'Demo Owner',
     '$2a$10$YT1plpOWpnZIrgNoPNeOi.eDN13s0/c/C5hRvakg0/LJwfkcjwiJW',
     'user', 'active'),
    ('00000000-0000-0000-0000-000000000102'::uuid,
     'viewer@demo.local', 'Demo Viewer',
     '$2a$10$YT1plpOWpnZIrgNoPNeOi.eDN13s0/c/C5hRvakg0/LJwfkcjwiJW',
     'user', 'active'),
    ('00000000-0000-0000-0000-000000000103'::uuid,
     'admin@admin.ru', 'Platform Admin',
     '$2a$10$YT1plpOWpnZIrgNoPNeOi.eDN13s0/c/C5hRvakg0/LJwfkcjwiJW',
     'admin', 'active')
) AS d(id, email, name, password_hash, role, status)
CROSS JOIN tenants t
WHERE t.slug = 'default'
ON CONFLICT (email) DO NOTHING;

-- Демо-проект.
INSERT INTO projects (id, tenant_id, owner_id, name, description, status)
SELECT
    '00000000-0000-0000-0000-000000000201'::uuid,
    t.id,
    '00000000-0000-0000-0000-000000000101'::uuid,
    'Демо-лестница', 'Пример проектирования лестницы (seed)', 'draft'
FROM tenants t
WHERE t.slug = 'default'
ON CONFLICT (id) DO NOTHING;

-- Членство: владелец — owner; второй — viewer (ровно один owner на проект).
INSERT INTO project_members (project_id, user_id, role)
VALUES
    ('00000000-0000-0000-0000-000000000201',
     '00000000-0000-0000-0000-000000000101', 'owner'),
    ('00000000-0000-0000-0000-000000000201',
     '00000000-0000-0000-0000-000000000102', 'viewer')
ON CONFLICT (project_id, user_id) DO NOTHING;

-- Демо-конфигурация (ревизия 1) — валидные параметры прямого марша.
INSERT INTO stair_configurations (
    id, project_id, revision,
    width_mm, height_mm, flight,
    step_height_mm, stringer_thickness_mm, step_thickness_mm,
    clearance_mm, railing_height_mm, comfort_step_mm,
    landing_width_mm, lower_step_count, outer_radius_mm,
    riser, landing_depth_mm, room_width_mm, room_length_mm, approach_space_mm
)
VALUES (
    '00000000-0000-0000-0000-000000000301',
    '00000000-0000-0000-0000-000000000201', 1,
    900, 2700, 'straight',
    180, 50, 40,
    0, 950, 300,
    1100, 7, 0,
    TRUE, 0, 0, 0, 1000
)
ON CONFLICT (id) DO NOTHING;

-- Текущая (активная) ревизия проекта.
UPDATE projects
SET current_configuration_id = '00000000-0000-0000-0000-000000000301'
WHERE id = '00000000-0000-0000-0000-000000000201'
  AND current_configuration_id IS NULL;