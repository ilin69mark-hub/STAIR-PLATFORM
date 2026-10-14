-- Откат 000037: убираем серверное событие о выдаче прав.
-- ВНИМАНИЕ: записи user.grants_changed останутся в журнале и нарушат CHECK,
-- поэтому откат возможен только после их удаления или снятия ограничения.

ALTER TABLE audit_events DROP CONSTRAINT IF EXISTS audit_events_action_check;

DELETE FROM audit_events WHERE action = 'user.grants_changed';

ALTER TABLE audit_events
    ADD CONSTRAINT audit_events_action_check CHECK (action NOT IN ('user.grants_changed'));
