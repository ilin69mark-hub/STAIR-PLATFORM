-- Закупочный прайс покупных деталей ограждения (S-179).
--
-- Зачем. Ограждение — 29 покупных позиций на лестницу — было в пакете
-- производства, но не стоило в смете НИ РУБЛЯ: ставки по нему не
-- существовало ни в коде, ни в базе, и расчёт молча отдавал цену без
-- ограждения. Это прямой денежный дефект: менеджер выдавал заниженную смету.
--
-- Ключ — ВИД детали (part_kind), а не код позиции каталога. У каждого
-- экземпляра свой номер в спецификации (PST-01, PST-02, …), и прайс по
-- нему построить нельзя: стойка стоит столько-то независимо от того, сколько
-- её на марше.
--
-- Нет строки — действует встроенная запасная ставка 250 ₽/шт. Так же устроены
-- прайсы крепежа (15 ₽) и герметика (500 ₽): пустой справочник не должен
-- обнулять материал.

CREATE TABLE store_purchased_prices (
    tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    -- Вид детали из PartKind: rail_top | post | rigel | tip | flange | slip |
    -- deco | end_cap. Справочник живёт в коде
    -- (dommfg.PurchasedParts()), и сверка таблицы с каталогом закрыта тестом.
    part_kind  TEXT NOT NULL CHECK (length(part_kind) > 0),
    -- Цена за ШТУКУ. Покупные детали не раскраиваются и не взвешиваются,
    -- поэтому цена за кг из store_rates к ним неприменима.
    price_per_pc_rub BIGINT NOT NULL CHECK (price_per_pc_rub > 0),
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, part_kind)
);

COMMENT ON TABLE store_purchased_prices IS
    'Закупочный прайс покупных деталей ограждения: цена за штуку по ВИДУ детали. '
    'Нет строки — действует встроенная ставка 250 ₽/шт';
COMMENT ON COLUMN store_purchased_prices.part_kind IS
    'Вид детали из PartKind (rail_top, post, rigel, tip, flange, slip, deco, end_cap), '
    'а не код экземпляра из спецификации';
COMMENT ON COLUMN store_purchased_prices.price_per_pc_rub IS
    'Цена за штуку. Ноль запрещён: нулевая цена означала бы «бесплатное '
    'ограждение» с ответом 200';