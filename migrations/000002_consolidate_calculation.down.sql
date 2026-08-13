-- Откат 000002: восстанавливаем отдельные JSONB-представления этапов.
-- Данные не сохранялись в них (канонический снапшот в result), поэтому
-- колонки восстанавливаются с NOT NULL, но без бэкап-переноса.

ALTER TABLE calculations
    ADD COLUMN flight        JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN geometry      JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN manufacturing JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN pricing       JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE calculations
    ALTER COLUMN flight        DROP DEFAULT,
    ALTER COLUMN geometry      DROP DEFAULT,
    ALTER COLUMN manufacturing DROP DEFAULT,
    ALTER COLUMN pricing       DROP DEFAULT;