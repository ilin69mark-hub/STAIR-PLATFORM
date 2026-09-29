-- Откат DB-001: снимаем добавленные ограничения.
--
-- ВНИМАНИЕ: после отката снова становятся возможны
--   * два эндпоинта одного вида в tenant (доставка уйдёт в более старый);
--   * произвольные status проекта и flight конфигурации;
--   * нулевые/отрицательные геометрические величины и суммы платежа.
-- Откатывать нужно вместе с кодом, который на эти инварианты опирается.

ALTER TABLE integration_events
    DROP CONSTRAINT IF EXISTS integration_events_attempts_check;

ALTER TABLE payment_intents
    DROP CONSTRAINT IF EXISTS payment_intents_amount_check;

ALTER TABLE stair_configurations
    DROP CONSTRAINT IF EXISTS stair_configurations_positive_geometry,
    DROP CONSTRAINT IF EXISTS stair_configurations_flight_check;

ALTER TABLE projects
    DROP CONSTRAINT IF EXISTS projects_status_check;

ALTER TABLE integration_endpoints
    DROP CONSTRAINT IF EXISTS integration_endpoints_tenant_kind_key;
