-- Откат SEC-002: снимаем CHECK-ограничения журнала аудита.
--
-- ВНИМАНИЕ (MIG-004, та же оговорка у 000002): откат намеренно делает
-- колонки nullable и БЕЗ default. Возврат колонок как NOT NULL без
-- default создаёт ловушку, блокирующую INSERT (см. 000002.down.sql).
--
-- Данные не удаляются: CHECK-ограничения исходно наложены на существующие
-- строки, а «историческая» подделка (если она была создана до фикса)
-- сохраняется — это правильно для журнала безопасности: удалять
-- доказательства нельзя.
ALTER TABLE audit_events
    DROP CONSTRAINT IF EXISTS audit_events_len_check;
ALTER TABLE audit_events
    DROP CONSTRAINT IF EXISTS audit_events_action_check;
ALTER TABLE audit_events
    DROP CONSTRAINT IF EXISTS audit_events_result_check;
