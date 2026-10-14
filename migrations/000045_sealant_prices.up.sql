-- 000045_sealant_prices — прайс герметика (CONF-SEALANT).
--
-- Полный аналог store_fastener_prices для крепежа: цена позиции каталога
-- герметика за штуку (картридж), задаётся магазином, без строки действует
-- встроенная ставка 500 ₽ (см. engine/pricing.DefaultSealantPriceRub).
--
-- Зачем отдельная таблица, а не колонка в store_rates: единица другая.
-- store_rates — это ₽/кг для листового и пиломатериала, а герметик покупают
-- картриджами. Общая колонка «цена» означала бы то одно, то другое в
-- зависимости от кода, и любая правка одного прайса ломала бы другой.
--
-- Состав позиций задаёт каталог движка
-- (internal/domain/manufacturing.DefaultSealantRegistry), а не эта таблица:
-- магазин может ТОЛЬКО переопределить цену, как и у материалов и крепежа.
-- Поэтому добавление новой позиции герметика не требует миграции.

CREATE TABLE store_sealant_prices (
    tenant_id                UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    sealant_code             TEXT NOT NULL CHECK (length(sealant_code) > 0),
    -- Цена за картридж. Картриджей может быть 310 или 600 мл — это всё равно
    -- «штука», поэтому колонка называется по единице закупки, а не по объёму.
    price_per_cartridge_rub  BIGINT NOT NULL CHECK (price_per_cartridge_rub >= 0),
    updated_by               UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, sealant_code)
);

COMMENT ON TABLE store_sealant_prices IS
    'Прайс герметика tenant: цена за картридж по коду позиции каталога CONF-SEALANT. Нет строки — действует встроенная ставка 500 ₽';