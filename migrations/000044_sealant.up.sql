-- 000044_sealant — герметик в расчёте ревизии (CONF-SEALANT).
--
-- Зачем. Герметик идёт по швам между деталями, и расход считается по
-- формуле владельца: V = S × t, где S — площадь герметизируемых стыков в
-- м², t — толщина слоя в мм. При плотности силикона 1,04 г/см³ слой 1 мм на
-- 1 м² даёт ≈1,05 кг, то есть ≈1 литр.
--
-- Площадь считается из геометрии обоймы выреза (стыки ступени с пяткой 1,
-- подступенка с пяткой 2 и с пластиной 1-й ступени) плюс торцы косоуров.
-- Сама площадь в БД не хранится — она производная от размеров лестницы.
--
-- Хранятся две величины:
--   sealant_layer_mm         — толщина слоя, мм. Умножается на площадь
--     напрямую, поэтому 2 мм против 1 мм удваивают и расход, и цену. 0 →
--     дефолт 1 мм.
--   extra_sealant_cartridges — доп. картриджи по объекту. Автоматический
--     расход округляется вверх до целого картриджа, и на неровном полу или
--     широких швах его не хватает; эта величина докупает сверх.
--
-- CHECK запрещает отрицательные значения: отрицательная толщина дала бы
-- отрицательный объём, и в смете появился бы герметик «с минусом».
--
-- Отдельная миграция, а не дополнение к 000043_fixing_fasteners: та уже
-- применена на стендах, и дописывание в применённую миграцию означало бы,
-- что её содержимое зависит от того, когда её запускали.

ALTER TABLE stair_configurations
    ADD COLUMN IF NOT EXISTS sealant_layer_mm DOUBLE PRECISION NOT NULL DEFAULT 0;

ALTER TABLE stair_configurations
    ADD COLUMN IF NOT EXISTS extra_sealant_cartridges INT NOT NULL DEFAULT 0;

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_sealant_check;
ALTER TABLE stair_configurations
    ADD CONSTRAINT stair_configurations_sealant_check
    CHECK (sealant_layer_mm >= 0 AND extra_sealant_cartridges >= 0);

COMMENT ON COLUMN stair_configurations.sealant_layer_mm IS
    'Толщина слоя герметика, мм. 0 = дефолт 1 мм. Расход = площадь швов × толщина';
COMMENT ON COLUMN stair_configurations.extra_sealant_cartridges IS
    'Дополнительные картриджи герметика по объекту сверх автоматического расхода';