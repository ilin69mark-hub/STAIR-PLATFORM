-- STAIR PLATFORM — Initial schema (MVP-08)
-- DB-0002 Database Model: Project → Stair Configuration → Calculation (result).
-- Все идентификаторы — UUID; временные метки — UTC.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Проект — корневая сущность (BC-001).
CREATE TABLE projects (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    status      TEXT        NOT NULL DEFAULT 'draft',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Конфигурация лестницы — единственный источник истины геометрии (BC-002).
-- Параметры хранятся в мм; angle — в радианах (ADR-0008).
CREATE TABLE stair_configurations (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id         UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    width_mm           DOUBLE PRECISION NOT NULL,
    height_mm          DOUBLE PRECISION NOT NULL,
    flight             TEXT        NOT NULL,
    step_height_mm     DOUBLE PRECISION NOT NULL,
    stringer_thickness_mm DOUBLE PRECISION NOT NULL,
    step_thickness_mm  DOUBLE PRECISION NOT NULL,
    clearance_mm       DOUBLE PRECISION NOT NULL,
    railing_height_mm  DOUBLE PRECISION NOT NULL,
    comfort_step_mm    DOUBLE PRECISION NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Результат расчёта — снапшот полного конвейера (Solver → Validation →
-- Geometry → Manufacturing → Cost → Price). Детерминирован: один и тот же
-- вход даёт один и тот же результат; снапшот фиксирует ревизию результата.
CREATE TABLE calculations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    configuration_id UUID       NOT NULL REFERENCES stair_configurations(id) ON DELETE CASCADE,
    valid           BOOLEAN     NOT NULL,
    blocking        BOOLEAN     NOT NULL,
    flight          JSONB       NOT NULL,
    geometry        JSONB       NOT NULL,
    manufacturing   JSONB       NOT NULL,
    pricing         JSONB       NOT NULL,
    result          JSONB       NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Индексы.
CREATE INDEX idx_stair_configurations_project ON stair_configurations(project_id);
CREATE INDEX idx_calculations_project ON calculations(project_id);
CREATE INDEX idx_calculations_configuration ON calculations(configuration_id);

-- Триггер обновления updated_at.
CREATE OR REPLACE FUNCTION touch_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_projects_touch BEFORE UPDATE ON projects
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER trg_stair_configurations_touch BEFORE UPDATE ON stair_configurations
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
