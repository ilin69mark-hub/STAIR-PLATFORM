-- STAIR PLATFORM — Payments (EDR-0027, Phase E E6).
-- Модель платёжных интентов: принимаем оплату за проект через внешний PSP
-- (mock-эмулятор или реальный). Входящий webhook верифицируется HMAC-SHA256
-- (тот же формат, что EDR-0023 §3.1/§3.2).
--
-- payment_intents: один платёжный интент. amount_minor — сумма в минимальных
--   денежных единицах (ADR-0008). provider + provider_checkout_id уникальны —
--   идемпотентность повторных webhook.
-- payment_events: журнал входящих событий webhook (аудит, EDR-0013).
--   intent_id каскадно: событие не переживает удаление интента.

CREATE TABLE payment_intents (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    project_id            UUID REFERENCES projects(id) ON DELETE SET NULL,
    user_id               UUID REFERENCES users(id) ON DELETE SET NULL,
    amount_minor          BIGINT NOT NULL,
    currency              TEXT NOT NULL DEFAULT 'USD',
    status                TEXT NOT NULL DEFAULT 'pending'
                          CHECK (status IN ('pending','paid','failed','refunded')),
    provider              TEXT NOT NULL,
    provider_checkout_id  TEXT NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at               TIMESTAMPTZ
);

CREATE UNIQUE INDEX payment_intents_provider_checkout_uniq
    ON payment_intents (provider, provider_checkout_id);
CREATE INDEX payment_intents_tenant_idx ON payment_intents (tenant_id);
CREATE INDEX payment_intents_project_idx ON payment_intents (project_id);
CREATE INDEX payment_intents_status_idx ON payment_intents (status);

CREATE TABLE payment_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    intent_id   UUID NOT NULL REFERENCES payment_intents(id) ON DELETE CASCADE,
    event_type  TEXT NOT NULL,
    payload     JSONB NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX payment_events_tenant_idx ON payment_events (tenant_id);
CREATE INDEX payment_events_intent_idx ON payment_events (intent_id);