-- STAIR PLATFORM — Enterprise Controls (EDR-0016, Phase G G4).
-- Политики безопасности per-tenant (JSONB) и API-ключи для интеграций.
--
-- tenant_settings: одна строка на tenant, JSONB-поле policy
--   {min_password_length, require_number, require_upper,
--    session_ttl_seconds, login_rate_limit_per_min}.
-- api_keys: долгоживущие service-токены; в БД хранится только SHA-256 хеш,
--   открытый токен отдаётся один раз при создании. Отзыв — мягкий (revoked_at).

CREATE TABLE tenant_settings (
    tenant_id  UUID PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    policy     JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE api_keys (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    scopes       TEXT[] NOT NULL DEFAULT '{}',
    created_by   UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at   TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ
);

CREATE INDEX api_keys_tenant_idx ON api_keys (tenant_id);
