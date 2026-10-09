-- 000040_drop_spiral_flight — винтовой марш удалён из проекта (решение
-- владельца 2026-10-06).
--
-- Что было. `stair_configurations.flight` допускал четыре значения:
-- straight, l_shape, u_shape, spiral. Спираль была единственным самым
-- компактным типом марша и держалась на трёх опорах: солвер
-- (solver/spiral.go), геометрия (веер из секторов с центральной колонной)
-- и фича-флаг S-152 в HTTP-слое, который скрывал её от публичного API.
-- Код, каталог материалов и тесты удалены.
--
-- Что меняется в схеме:
--   1. CHECK на flight сужается до трёх оставшихся типов.
--   2. Колонка spiral_direction (из миграции 000030) удаляется вместе со
--      своим CHECK — приложение её больше ни читает, ни пишет.
--   3. Колонка outer_radius_mm удаляется: наружный радиус спирали больше
--      не имеет смысла ни в одном марше.
--
-- ПОРЯДОК ШАГОВ ЗНАЧИМ. Сначала сужается CHECK, потом удаляются колонки.
-- Если в базе есть строки `flight = 'spiral'`, ADD CONSTRAINT провалится по
-- ним — и это сделано НАМЕРЕННО, до потери колонок: миграция падает целиком
-- (golang-migrate выполняет файл в одной транзакции), ничего не теряется.
--
-- Более того, до миграции срабатывает preflight из migguard: он read-only
-- печатает id нарушителей и завершает `migrate` с кодом 1, НЕ пачкая версию.
-- То есть оператор видит проблему до того, как она станет необратимой:
--
--   go run ./cmd/migrate -preflight
--
-- Если преflight нашёл винтовые конфигурации, осознанное решение оператора —
-- перевести их на ближайший прямой марш или удалить проекты. Молча
-- подменять тип марша нельзя: у винтовой лестницы своя геометрия, и
-- пересчёт под прямым даст другое изделие.

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_flight_check;

ALTER TABLE stair_configurations
    ADD CONSTRAINT stair_configurations_flight_check
    CHECK (flight IN ('straight', 'l_shape', 'u_shape'));

-- Колонки спирали. CHECK на spiral_direction уходит вместе с колонкой, но
-- DROP IF EXISTS делает шаг независимым от порядка и идемпотентным.
ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_spiral_direction_check;

ALTER TABLE stair_configurations
    DROP COLUMN IF EXISTS spiral_direction,
    DROP COLUMN IF EXISTS outer_radius_mm;
