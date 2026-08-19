-- STAIR PLATFORM — Retail Orders (клиентский сайт, Store).
-- Модель розничных заказов-лидов: клиент собирает геометрию на публичном
-- конструкторе, рассчитывает предварительную цену и отправляет заказ.
--   config_json — снимок входной конфигурации (calculateRequest);
--   price_json  — снимок цены публичного расчёта (без производственного пакета);
--   project_id  — опциональная связь с проектом в инженерной панели
--                (менеджер ведёт заказ дальше).

CREATE TABLE orders (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status       TEXT NOT NULL DEFAULT 'new'
                 CHECK (status IN ('new','priced','confirmed','in_progress','completed','cancelled')),
    contact      JSONB NOT NULL,
    config_json  JSONB NOT NULL,
    price_json   JSONB NOT NULL,
    project_id   UUID REFERENCES projects(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX orders_tenant_idx ON orders (tenant_id);
CREATE INDEX orders_user_idx ON orders (user_id);
CREATE INDEX orders_status_idx ON orders (status);
CREATE INDEX orders_project_idx ON orders (project_id);