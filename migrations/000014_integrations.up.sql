-- STAIR PLATFORM — Integrations (EDR-0023, Phase E E2).
-- Фундамент webhook-платформы: регистрация интеграционных эндпоинтов
-- (ERP/CRM/MES/Payments) и журнал событий доставки.
--
-- integration_endpoints: webhook-эндпоинт tenant'а. kind — тип интеграции
--   (erp|crm|mes|payment). secret_enc — HMAC-секрет (приложение отвечает
--   за его не-попадание в логи/JSON).
-- integration_events: одно событие доставки. payload — JSONB с телом
--   события (для erp.quote_send — snapshot проекта). status:
--   pending|delivered|failed|dlq.

CREATE TABLE integration_endpoints (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL CHECK (kind IN ('erp','crm','mes','payment')),
    url         TEXT NOT NULL,
    secret_enc  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX integration_endpoints_tenant_idx ON integration_endpoints (tenant_id);

CREATE TABLE integration_events (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    endpoint_id  UUID NOT NULL REFERENCES integration_endpoints(id) ON DELETE CASCADE,
    project_id   UUID REFERENCES projects(id) ON DELETE SET NULL,
    event_type   TEXT NOT NULL,
    payload      JSONB NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','delivered','failed','dlq')),
    attempts     INT NOT NULL DEFAULT 0,
    last_error   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ
);

CREATE INDEX integration_events_tenant_idx ON integration_events (tenant_id);
CREATE INDEX integration_events_endpoint_idx ON integration_events (endpoint_id);
CREATE INDEX integration_events_status_idx ON integration_events (status);