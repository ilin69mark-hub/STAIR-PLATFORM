-- STAIR PLATFORM — Store settings и прайс магазина (волна 0 admin store).
--
-- Настройки магазина: контакты, реквизиты, соцсети, SEO и параметры
-- расчёта (ставки станка/труда, накладные, маржа, скидка, НДС). Ровно одна
-- строка на tenant; настройки, которых не задали, берутся из дефолтов сервиса.
--
-- Цена материала (₽/кг) живёт отдельной таблицей store_rates: это
-- редактируемая часть каталога MFG-0005, а не код материала. Нет строки —
-- действует зашитая в движок ставка (DefaultRates), поэтому новый материал
-- не требует миграции.
--
-- Всё tenant-scoped: это же станет основой мультитенанта (ROADMAP-0013).

CREATE TABLE store_settings (
    tenant_id   UUID PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    settings    JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE store_settings IS
    'Настройки публичного магазина tenant: контакты, реквизиты, соцсети, SEO, параметры расчёта';

CREATE TABLE store_rates (
    tenant_id          UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    material_code      TEXT NOT NULL,
    price_per_kg_rub   BIGINT NOT NULL CHECK (price_per_kg_rub >= 0),
    updated_by         UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, material_code)
);

COMMENT ON TABLE store_rates IS
    'Цена материала магазина, ₽/кг; переопределяет DefaultRates для расчётов tenant';

CREATE INDEX store_rates_tenant_idx ON store_rates (tenant_id);
