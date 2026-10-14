-- 000034_tread_material — разделение материала каркаса и материала ступеней.
--
-- ДО разделения у лестницы был ОДИН материал на все детали. Это физически
-- невозможно для металлокаркаса с деревянными ступенями: стальной косоур
-- бывает 6–10 мм, а проступь из дуба — от 20 мм (MinThickness в каталоге
-- MFG-0005). Одно поле не описывало оба случая, и валидация отвергала
-- стальной каркас с деревянной проступью.
--
-- Добавляем:
--   tread_material_code — материал ступеней (проступи, площадки, поворотные
--     ступени). NULL/"" → наследуется от material_code, то есть поведение
--     существующих строк не меняется.
--   riser_thickness_mm  — толщина подступенка. Раньше подступенок строился
--     толщиной проступи, потому что материал был один. Теперь подступенок —
--     деталь каркаса и может быть тонким. 0/NULL → наследуется от
--     step_thickness_mm.
--
-- Каталог материалов живёт в коде (engine/manufacturing), поэтому CHECK со
-- списком кодов был бы ложью и сломался бы при добавлении материала. Как и
-- для material_code — проверка непустой строки; конкретный код валидирует
-- application-слой при расчёте.

ALTER TABLE stair_configurations
    ADD COLUMN IF NOT EXISTS tread_material_code TEXT;

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_tread_material_check;
ALTER TABLE stair_configurations
    ADD CONSTRAINT stair_configurations_tread_material_check
    CHECK (tread_material_code IS NULL OR length(tread_material_code) > 0);

ALTER TABLE stair_configurations
    ADD COLUMN IF NOT EXISTS riser_thickness_mm DOUBLE PRECISION NOT NULL DEFAULT 0;

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_riser_thickness_check;
ALTER TABLE stair_configurations
    ADD CONSTRAINT stair_configurations_riser_thickness_check
    CHECK (riser_thickness_mm IS NULL OR riser_thickness_mm >= 0);

COMMENT ON COLUMN stair_configurations.tread_material_code IS
    'Код материала ступеней MFG-0005 (напр. WOOD-OAK). NULL = наследуется от material_code (вся лестница из одного материала)';
COMMENT ON COLUMN stair_configurations.riser_thickness_mm IS
    'Толщина подступенка, мм. 0 = наследуется от step_thickness_mm (поведение до разделения материалов)';
