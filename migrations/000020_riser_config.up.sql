-- Подступенки: параметр конфигурации (BC-002). По умолчанию TRUE —
-- существующие конфигурации не меняют своего поведения (закрытые ступени).
ALTER TABLE stair_configurations
    ADD COLUMN riser BOOLEAN NOT NULL DEFAULT TRUE;