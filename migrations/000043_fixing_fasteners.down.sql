-- Откат 000043_fixing_fasteners.
--
-- ВНИМАНИЕ: откат УДАЛЯЕТ основания крепления и доп. крепёж из сохранённых
-- ревизий, а также весь прайс крепежа. Откатывать имеет смысл только вместе
-- с откатом кода, который их читает: после отката расчёт снова будет
-- считать крепёж по умолчанию (бетон, без доп. позиций), и смета проекта
-- перестанет совпадать с той, по которой его утверждали.

DROP TABLE IF EXISTS store_fastener_prices;

ALTER TABLE stair_configurations DROP COLUMN IF EXISTS is_measurement;

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_extra_fasteners_check;
ALTER TABLE stair_configurations DROP COLUMN IF EXISTS extra_fasteners;

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_ceiling_fixing_check;
ALTER TABLE stair_configurations DROP COLUMN IF EXISTS ceiling_fixing;

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_floor_fixing_check;
ALTER TABLE stair_configurations DROP COLUMN IF EXISTS floor_fixing;