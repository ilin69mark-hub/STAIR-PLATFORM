-- STAIR PLATFORM — SSO (EDR-0017, Phase G G5).
-- oauth_accounts: привязка внешнего identity (provider+subject) к
-- учётной записи users. UNIQUE(provider, subject): один внешний identity
-- — одна учётная запись.
-- sso_states: одноразовые OIDC-состояния для начала входа (state + nonce +
-- PKCE verifier). state_hash — SHA-256 от state (как sessions).

CREATE TABLE oauth_accounts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider   TEXT NOT NULL,
    subject    TEXT NOT NULL,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, subject)
);

CREATE INDEX oauth_accounts_user_idx ON oauth_accounts (user_id);

CREATE TABLE sso_states (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    state_hash    TEXT NOT NULL UNIQUE,
    nonce         TEXT NOT NULL,
    pkce_verifier TEXT NOT NULL,
    redirect      TEXT NOT NULL DEFAULT '/',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL
);