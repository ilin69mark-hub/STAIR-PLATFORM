-- Откат 000044_sealant.
--
-- ВНИМАНИЕ: откат УДАЛЯЕТ толщину слоя и доп. картриджи из сохранённых
-- ревизий. Откатывать имеет смысл только вместе с откатом кода, который их
-- читает: после отката расчёт вернётся к дефолтному слою 1 мм без доп.
-- картриджей, и смета проекта перестанет совпадать с утверждённой.

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_sealant_check;
ALTER TABLE stair_configurations DROP COLUMN IF EXISTS extra_sealant_cartridges;
ALTER TABLE stair_configurations DROP COLUMN IF EXISTS sealant_layer_mm;