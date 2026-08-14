-- STAIR PLATFORM — MVP-11 (F2): Identity, AuthN/AuthZ, Tenant isolation.
-- SEC-0003 Identity/Authentication, SEC-0004 Authorization,
-- SEC-0005 Tenant Isolation.
--
-- Вводим tenants (базовая граница изоляции данных), users (учётные записи
-- с bcrypt-хешем пароля) и sessions (серверные сессии: в БД хранится
-- SHA-256 хеш opaque-токена). Существующие проекты переводятся в дефолтный
-- tenant, чтобы изоляция была бесшовной для уже сохранённых данных.

-- Tenants — граница изоляции данных (SEC-0005).
CREATE TABLE tenants (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    slug       TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO tenants (name, slug) VALUES ('Default', 'default');

-- Все существующие проекты принадлежат дефолтному tenant'у.
ALTER TABLE projects ADD COLUMN tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;
UPDATE projects SET tenant_id = (SELECT id FROM tenants WHERE slug = 'default');
ALTER TABLE projects ALTER COLUMN tenant_id SET NOT NULL;
CREATE INDEX projects_tenant_id_idx ON projects(tenant_id);

-- Users — учётные записи (SEC-0003). Email уникален на платформе.
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email         TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
    status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX users_tenant_id_idx ON users(tenant_id);

-- Sessions — серверные сессии; в БД хранится только хеш токена.
-- Ревокация мгновенная (удаление строки); истёкшие сессии отбраковываются
-- по expires_at.
CREATE TABLE sessions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX sessions_user_id_idx ON sessions(user_id);
CREATE INDEX sessions_expires_idx ON sessions(expires_at);
