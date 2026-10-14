-- Свои позиции крепежа для tenant'а (решение владельца 2026-10-13).
--
-- Зачем. Состав каталога крепежа был зашит в код
-- (dommfg.DefaultFastenerRegistry, 42 позиции). Позиция, которой в коде
-- нет, в смету попасть не могла: движок молча её пропускал, и менеджер
-- получал заниженную смету. Найти новый крепёж значило править Go и
-- пересобирать сервер.
--
-- Теперь позицию добавляет владелец в админке «Магазин → Крепёж»: код,
-- название, вид, диаметр, длина, материал и цена. Позиция попадает в
-- каталог tenant'а и в его сметы, но НЕ в чужие: таблица привязана к
-- tenant_id, как и прайс.
--
-- Что НЕ меняется: встроенный каталог остаётся источником правды по
-- умолчанию, и добавленная позиция не может его переопределить — код,
-- совпавший со встроенным, отклоняется. Иначе одна позиция, названная
-- SCREW-6x35 с другим размером, тихо испортила бы расчёт всем.

CREATE TABLE store_fastener_positions (
    tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    -- Код позиции: тот же формат, что у встроенных (SCREW-6x35, PLUG-10x60).
    -- Он же ключ в store_fastener_prices — цена позиции лежит там же, где у
    -- встроенных, и отдельного прайса для своих позиций не заводится.
    fastener_code TEXT NOT NULL CHECK (length(btrim(fastener_code)) > 0),
    name_ru       TEXT NOT NULL CHECK (length(btrim(name_ru)) > 0),
    -- screw (глухарь/саморез) | plug (дюбель). Других видов нет: FastenerKind
    -- их не знает, и позиция другого вида молча выпала бы из сметы.
    kind          TEXT NOT NULL CHECK (kind IN ('screw', 'plug')),
    diameter_mm   NUMERIC(10, 2) NOT NULL CHECK (diameter_mm > 0),
    length_mm     NUMERIC(10, 2) NOT NULL CHECK (length_mm > 0),
    -- Материал, в который крепёж вворачивают: '' (не задан) | concrete |
    -- wood | metal. Совпадает с FastenerBase. Пустое значение — НЕ бетон,
    -- как у FixingBase: тут это «основание не задано».
    base          TEXT NOT NULL DEFAULT '' CHECK (base IN ('', 'concrete', 'wood', 'metal')),
    created_by    UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_by    UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, fastener_code),
    -- Диаметр и длина ограничены сверху величинами, до которых разумно
    -- дорастает метиз. Без предела в таблицу влезает «глухарь 1000000х5»,
    -- и ошибка ввода уедет в смету как настоящая позиция.
    CONSTRAINT store_fastener_positions_size_ck
        CHECK (diameter_mm <= 50 AND length_mm <= 400)
);

COMMENT ON TABLE store_fastener_positions IS
    'Свои позиции крепежа tenant: код, название, вид, размер, материал. '
    'Дополняют встроенный каталог dommfg.DefaultFastenerRegistry(), не заменяют его';
COMMENT ON COLUMN store_fastener_positions.fastener_code IS
    'Код позиции, тот же ключ, что в store_fastener_prices: цена своей позиции '
    'хранится там же, где цена встроенной';
COMMENT ON COLUMN store_fastener_positions.base IS
    'Материал вворачивания: "" | concrete | wood | metal. Пустое — «не задан», '
    'а НЕ бетон (в FixingBase пустое означает бетон, здесь такого подменять нельзя)';

CREATE INDEX store_fastener_positions_tenant_idx
    ON store_fastener_positions (tenant_id);
