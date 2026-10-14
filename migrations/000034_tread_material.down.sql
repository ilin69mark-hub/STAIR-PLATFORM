-- Откат 000034_tread_material. Восстанавливает поведение «один материал на
-- всю лестницу» и «подступенок толщиной проступи».

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_riser_thickness_check;
ALTER TABLE stair_configurations
    DROP COLUMN IF EXISTS riser_thickness_mm;

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_tread_material_check;
ALTER TABLE stair_configurations
    DROP COLUMN IF EXISTS tread_material_code;
