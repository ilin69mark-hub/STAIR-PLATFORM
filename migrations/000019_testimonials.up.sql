-- STAIR PLATFORM — Отзывы клиентов (store) для админки и лендинга.
-- Менеджер публикует отзывы (published), лендинг показывает только их.
CREATE TABLE testimonials (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    author     TEXT NOT NULL,
    text       TEXT NOT NULL,
    rating     SMALLINT NOT NULL
               CHECK (rating BETWEEN 1 AND 5),
    published  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX testimonials_tenant_idx ON testimonials (tenant_id);
CREATE INDEX testimonials_published_idx ON testimonials (tenant_id) WHERE published;