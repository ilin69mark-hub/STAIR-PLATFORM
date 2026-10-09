-- Откат 000040: возвращаем винтовой марш в схему.
--
-- ВНИМАНИЕ: откат ВОЗВРАЩАЕТ ЧЕТЫРЕ значения flight и две колонки, но НЕ
-- возвращает код. Спираль удалена из солвера, геометрии, каталога и тестов,
-- поэтому после отката база будет принимать `flight = 'spiral'`, а приложение
-- отдаст на такой конфигурации блокирующую «Неизвестный тип лестницы».
--
-- Откат имеет смысл только вместе с откатом кода. Как отдельная операция он
-- НЕ возвращает значения: DROP COLUMN удалил их вместе с колонками, и
-- восстановить наружный радиус и закрутку можно только из бэкапа базы,
-- снятого до применения 000040. Колонки восстанавливаются пустыми
-- (outer_radius_mm — с DEFAULT 0), чтобы схема совпала с кодом, который
-- больше их не читает.

ALTER TABLE stair_configurations
    ADD COLUMN IF NOT EXISTS spiral_direction TEXT;

ALTER TABLE stair_configurations
    ADD COLUMN IF NOT EXISTS outer_radius_mm DOUBLE PRECISION NOT NULL DEFAULT 0;

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_spiral_direction_check;
ALTER TABLE stair_configurations
    ADD CONSTRAINT stair_configurations_spiral_direction_check
    CHECK (spiral_direction IS NULL OR spiral_direction IN ('cw', 'ccw'));

-- CHECK на flight расширяется ПОСЛЕ добавления колонок: он проверяет строки,
-- а не структуру, поэтому порядок здесь только для читаемости.
ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_flight_check;
ALTER TABLE stair_configurations
    ADD CONSTRAINT stair_configurations_flight_check
    CHECK (flight IN ('straight', 'l_shape', 'u_shape', 'spiral'));
