-- DOM-003 (forensic 2026-09-26): потеря 9 параметров конфигурации.
--
-- Проблема: `stair.Config` содержит 28 полей, но `stair_configurations`
-- хранил только 19. Девять полей не имели колонок ВООБЩЕ:
--
--   turn_kind          — тип поворота (площадка | поворотные ступени)
--   winder_count       — число поворотных ступеней (nw)
--   railing            — сторона перил прямого марша
--   railing_lower      — сторона перил первого марша L/U
--   railing_landing    — сторона перил площадки L/U
--   railing_upper      — сторона перил второго марша L/U
--   direction          — направление поворота площадки (left | right)
--   spiral_direction   — направление закрутки спирали (cw | ccw)
--   material_code      — материал конструкции (MFG-0005)
--
-- Последствия (воспроизведено):
--   1. CAD-экспорт (`project.ExportCAD` -> `fromConfigEntity`) пересобирал
--      конфигурацию из усечённой ревизии, то есть ВОЗВРАЩАЛ ДРУГУЮ
--      лестницу, чем рассчитал и утвердил пользователь.
--   2. Спираль теряла направление закрутки: `SpiralDirection("")` по
--      `DefaultRailing()` даёт RailingLeft, то есть у КАЖДОЙ сохранённой
--      спирали перила оказывались с левой стороны независимо от заданных.
--   3. `POST /projects/{id}/configurations/{id}/restore` активировал ревизию,
--      потерявшую тип поворота: U-образная лестница с поворотными
--      ступенями превращалась в лестницу с площадкой.
--   4. DOM-001: `turn_kind`/`winder_count` невозможно было даже передать
--      через API — вся функциональность поворотных ступеней была
--      недостижима извне.
--
-- Все колонки nullable: «не задано» — валидное состояние (пустое значение
-- кода трактуется как дефолт), поэтому DEFAULT/NOT NULL здесь были бы
-- ложью и сломали бы существующие строки. Исторические строки получат NULL,
-- что для всех полей означает ровно то поведение, которое было до миграции
-- (поле не задано -> дефолт). Данные НЕ переписываются.
--
-- CHECK-ограничения на enums добавлены с IF NOT EXISTS-подобной идемотентностью
-- (DROP IF EXISTS + ADD), чтобы миграция была повторно применима.

-- 1) Тип поворота и число поворотных ступеней (DOM-001, CONF-TURN-KIND).
ALTER TABLE stair_configurations
    ADD COLUMN IF NOT EXISTS turn_kind TEXT,
    ADD COLUMN IF NOT EXISTS winder_count INTEGER;

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_turn_kind_check;
ALTER TABLE stair_configurations
    ADD CONSTRAINT stair_configurations_turn_kind_check
    CHECK (turn_kind IS NULL OR turn_kind IN ('platform', 'winder'));

-- winder имеет смысл только вместе с turn_kind='winder' и не меньше 3
-- поворотных ступеней (устойчивость поворота на 180°, EDR-0006 §7).
ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_winder_check;
ALTER TABLE stair_configurations
    ADD CONSTRAINT stair_configurations_winder_check CHECK (
        (turn_kind IS NULL OR turn_kind = 'platform')
        AND (winder_count IS NULL OR winder_count = 0)
        OR (turn_kind = 'winder' AND winder_count IS NOT NULL AND winder_count >= 3)
    );

-- 2) Стороны перил (CONF-RAILING).
ALTER TABLE stair_configurations
    ADD COLUMN IF NOT EXISTS railing TEXT,
    ADD COLUMN IF NOT EXISTS railing_lower TEXT,
    ADD COLUMN IF NOT EXISTS railing_landing TEXT,
    ADD COLUMN IF NOT EXISTS railing_upper TEXT;

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_railing_check;
ALTER TABLE stair_configurations
    ADD CONSTRAINT stair_configurations_railing_check CHECK (
        (railing         IS NULL OR railing         IN ('none', 'left', 'right', 'both'))
    AND (railing_lower   IS NULL OR railing_lower   IN ('none', 'left', 'right', 'both'))
    AND (railing_landing IS NULL OR railing_landing IN ('none', 'left', 'right', 'both'))
    AND (railing_upper   IS NULL OR railing_upper   IN ('none', 'left', 'right', 'both'))
    );

-- 3) Направление поворота площадки (CONF-DIRECTION).
ALTER TABLE stair_configurations
    ADD COLUMN IF NOT EXISTS direction TEXT;

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_direction_check;
ALTER TABLE stair_configurations
    ADD CONSTRAINT stair_configurations_direction_check
    CHECK (direction IS NULL OR direction IN ('left', 'right'));

-- 4) Направление закрутки спирали (CONF-SPIRAL-DIRECTION).
ALTER TABLE stair_configurations
    ADD COLUMN IF NOT EXISTS spiral_direction TEXT;

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_spiral_direction_check;
ALTER TABLE stair_configurations
    ADD CONSTRAINT stair_configurations_spiral_direction_check
    CHECK (spiral_direction IS NULL OR spiral_direction IN ('cw', 'ccw'));

-- 5) Материал конструкции (MFG-0005).
--    Каталог живёт в коде (engine/manufacturing), поэтому CHECK со списком
--    был бы ложью (сломается при добавлении материала). Вместо этого
--    проверка непустой строки: '' невалиден, а конкретный код валидирует
--    application-слой при расчёте.
ALTER TABLE stair_configurations
    ADD COLUMN IF NOT EXISTS material_code TEXT;

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_material_check;
ALTER TABLE stair_configurations
    ADD CONSTRAINT stair_configurations_material_check
    CHECK (material_code IS NULL OR length(material_code) > 0);

COMMENT ON COLUMN stair_configurations.turn_kind IS
    'Тип поворота L/U: platform (по умолчанию) | winder (поворотные ступени). NULL = platform (обратная совместимость)';
COMMENT ON COLUMN stair_configurations.winder_count IS
    'Число поворотных ступеней nw (только при turn_kind=winder, >= 3). NULL/0 = нет';
COMMENT ON COLUMN stair_configurations.material_code IS
    'Код материала каталога MFG-0005 (напр. STEEL-S235). NULL = автоназначение по толщине';
