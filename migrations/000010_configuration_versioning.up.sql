-- STAIR PLATFORM — Configuration versioning (EDR-0012, Phase C C6).
-- Revision: монотонный номер ревизии конфигурации внутри проекта (DB-0006).
-- current_configuration_id: текущая (активная) ревизия проекта.
--
-- DB-003 (forensic 2026-09-24): колонка сначала добавляется БЕЗ default и
-- заполняется row_number() по каждому проекту, и только потом ставится
-- NOT NULL DEFAULT 1. Прежний вариант (`ADD COLUMN ... NOT NULL DEFAULT 1`)
-- проставлял 1 ВСЕМ существующим строкам → проекты с ≥2 конфигурациями
-- ломали создание UNIQUE-индекса (Key (project_id, revision) is duplicated),
-- то есть цикл down→up (rollback) был невозможен, а сбой up оставлял БД в
-- состоянии dirty 10 (это и был корень инцидентов S-137/Dirty version 10).

ALTER TABLE stair_configurations
    ADD COLUMN revision INTEGER;

UPDATE stair_configurations sc
   SET revision = ranked.rn
  FROM (SELECT id,
               ROW_NUMBER() OVER (PARTITION BY project_id ORDER BY created_at, id) AS rn
          FROM stair_configurations) AS ranked
 WHERE sc.id = ranked.id;

ALTER TABLE stair_configurations
    ALTER COLUMN revision SET DEFAULT 1;

ALTER TABLE stair_configurations
    ALTER COLUMN revision SET NOT NULL;

CREATE UNIQUE INDEX stair_configurations_project_revision_idx
    ON stair_configurations (project_id, revision);

ALTER TABLE projects
    ADD COLUMN current_configuration_id UUID
        REFERENCES stair_configurations(id) ON DELETE SET NULL;