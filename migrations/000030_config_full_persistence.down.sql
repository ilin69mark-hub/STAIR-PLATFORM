-- Откат DOM-003: убираем 9 добавленных колонок.
--
-- ВНИМАНИЕ: откат УДАЛЯЕТ сохранённые параметры конфигурации (тип
-- поворота, стороны перил, направления, материал). Откатывать имеет смысл
-- только вместе с откатом кода, который их использует; в противном случае
-- экспорт CAD вернётся к усечённой конфигурации (та дефект, который эта
-- миграция устраняет).
--
-- Колонки удаляются в обратном порядке добавления; CHECK-ограничения
-- удаляются явно (они уходят вместе с колонками, но DROP IF EXISTS делает
-- откат независимым от порядка и идемпотентным).

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_material_check;
ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_spiral_direction_check;
ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_direction_check;
ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_railing_check;
ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_winder_check;
ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_turn_kind_check;

ALTER TABLE stair_configurations
    DROP COLUMN IF EXISTS material_code,
    DROP COLUMN IF EXISTS spiral_direction,
    DROP COLUMN IF EXISTS direction,
    DROP COLUMN IF EXISTS railing_upper,
    DROP COLUMN IF EXISTS railing_landing,
    DROP COLUMN IF EXISTS railing_lower,
    DROP COLUMN IF EXISTS railing,
    DROP COLUMN IF EXISTS winder_count,
    DROP COLUMN IF EXISTS turn_kind;
