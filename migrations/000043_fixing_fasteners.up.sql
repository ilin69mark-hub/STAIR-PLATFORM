-- 000043_fixing_fasteners — основания крепления и прайс крепежа (MNT-0001,
-- решение владельца 2026-10-08).
--
-- Зачем. Пятки обоймы выреза давно просверливаются лазером вместе с
-- контуром (geometry/comb_heels.go), но крепёж — дюбели и глухари — в расчёт
-- не попадал: ManufacturingCostDataset.FastenerCount стоял нулём с момента
-- создания набора. Лестница из 238 отверстий обходилась как лестница без
-- единого самореза.
--
-- Две части:
--
--   1. Конфигурация ревизии:
--        floor_fixing   — крепление пятки пола: concrete | wood
--        ceiling_fixing — крепление пятки выхода на перекрытие: то же
--        extra_fasteners — доп. крепёж по объекту, JSON-массив кодов
--
--      NULL/"" → бетон: лестничные пятки крепят к перекрытию, а не к полу,
--      и поведение существующих строк не меняется. CHECK ограничен перечнем
--      значений, потому что здесь, в отличие от кодов материалов, набор
--      закрыт: основание крепления — это бетон или дерево, новых оснований
--      не бывает.
--
--      extra_fasteners НЕ ограничен CHECK: коды позиций задаются справочником,
--      который владелец редактирует прямо в админке. Связать колонку с
--      каталогом из кода — значит запретить владельцу добавить крепёж.
--
--   2. Прайс крепежа store_fastener_prices — то же, что store_rates для
--      материалов, но единица ШТУКА, а не кг. Без отдельной таблицы цена
--      дюбеля ехала бы в килограммах стали и давала бы полную стоимость
--      самореза.

ALTER TABLE stair_configurations
    ADD COLUMN IF NOT EXISTS floor_fixing TEXT;

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_floor_fixing_check;
ALTER TABLE stair_configurations
    ADD CONSTRAINT stair_configurations_floor_fixing_check
    CHECK (floor_fixing IS NULL OR floor_fixing IN ('concrete', 'wood'));

ALTER TABLE stair_configurations
    ADD COLUMN IF NOT EXISTS ceiling_fixing TEXT;

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_ceiling_fixing_check;
ALTER TABLE stair_configurations
    ADD CONSTRAINT stair_configurations_ceiling_fixing_check
    CHECK (ceiling_fixing IS NULL OR ceiling_fixing IN ('concrete', 'wood'));

-- JSONB-массив кодов. DEFAULT '[]', а не NULL: доп. крепёж по объекту —
-- норма, а не «неизвестно», и пустой список не должен читаться как
-- недопустимое значение.
ALTER TABLE stair_configurations
    ADD COLUMN IF NOT EXISTS extra_fasteners JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_extra_fasteners_check;
ALTER TABLE stair_configurations
    ADD CONSTRAINT stair_configurations_extra_fasteners_check
    CHECK (jsonb_typeof(extra_fasteners) = 'array');

--   3. Метка «замер»:
--        is_measurement     — ревизия снята по факту, а не по телефону
--
--      Зачем. Размеры приходят от клиента по телефону, потом уточняются на
--      объекте, и обе цифры остаются в истории проекта. По дате ревизии это
--      не различить: неделя между «примерно 900» и «ровно 937» не видна, а
--      разница меняет и число ступеней, и цену. Метку ставит тот, кто был
--      на замере.
ALTER TABLE stair_configurations
    ADD COLUMN IF NOT EXISTS is_measurement BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE store_fastener_prices (
    tenant_id      UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    fastener_code  TEXT NOT NULL CHECK (length(fastener_code) > 0),
    -- Цена за ШТУКУ. Отдельная единица, поэтому отдельная колонка: общая
    -- price_per_kg_rub из store_rates для крепежа бессмысленна.
    price_per_pc_rub BIGINT NOT NULL CHECK (price_per_pc_rub >= 0),
    updated_by     UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, fastener_code)
);

COMMENT ON COLUMN stair_configurations.is_measurement IS
    'Ревизия снята по факту на объекте (после выезда на замер), а не по телефону';

COMMENT ON TABLE store_fastener_prices IS
    'Прайс крепежа tenant: цена за штуку по коду позиции каталога MNT-0001. Нет строки — действует встроенная ставка 15 ₽';
COMMENT ON COLUMN stair_configurations.floor_fixing IS
    'Основание крепления пятки пола: concrete | wood. NULL = бетон';
COMMENT ON COLUMN stair_configurations.ceiling_fixing IS
    'Основание крепления пятки выхода на перекрытие: concrete | wood. NULL = бетон';
COMMENT ON COLUMN stair_configurations.extra_fasteners IS
    'Дополнительный крепёж по объекту: JSON-массив кодов каталога ("КОД" или "КОД×количество")';