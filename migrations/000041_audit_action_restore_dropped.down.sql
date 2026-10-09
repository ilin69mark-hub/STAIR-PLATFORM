-- Откат 000041: CHECK возвращается к перечню из 000039, то есть к тому,
-- который миграция 000038 подменила.
--
-- ГЛАВНОЕ — ПОЧЕМУ ЗДЕСЬ ЕСТЬ УДАЛЕНИЕ СТРОК.
--
-- Перечень 000039 уже, чем перечень 000041. Между применением 000041 и этим
-- откатом код успел записать в журнал события из десяти возвращённых значений
-- (project.modified, member.added, api_key.created и прочие): именно ради
-- них миграция и делалась. Простой `ADD CONSTRAINT ... CHECK (action IN (...))`
-- на таких данных ПАДАЕТ — PostgreSQL проверяет ограничение и на существующих
-- строках. Откат на этом встал бы, оставив базу в dirty, из которой, как
-- известно, `migrate up` больше не выйдет.
--
-- Удалять такие строки молча нельзя: это журнал безопасности. Поэтому они
-- сначала КОПИРУЮТСЯ в audit_events_action_restore_backup, и только затем
-- удаляются из audit_events. Откат остаётся обратимым: вернуть строки можно
--
--   INSERT INTO audit_events
--       SELECT * FROM audit_events_action_restore_backup
--    WHERE id NOT IN (SELECT id FROM audit_events);
--
-- а сама резервная таблица удаляется отдельной миграцией, когда оператор
-- убедится, что она больше не нужна. Пустой резерв (обычный случай: журнал
-- не успел наполниться) ничего не меняет.

CREATE TABLE IF NOT EXISTS audit_events_action_restore_backup (
    LIKE audit_events INCLUDING ALL
);

INSERT INTO audit_events_action_restore_backup
SELECT * FROM audit_events
WHERE action IN (
    'api_key.created',
    'api_key.revoked',
    'data.exported',
    'member.added',
    'member.removed',
    'member.role_changed',
    'payment.refunded',
    'project.modified',
    'review.changes',
    'review.requested'
)
ON CONFLICT (id) DO NOTHING;

DELETE FROM audit_events
WHERE action IN (
    'api_key.created',
    'api_key.revoked',
    'data.exported',
    'member.added',
    'member.removed',
    'member.role_changed',
    'payment.refunded',
    'project.modified',
    'review.changes',
    'review.requested'
);

ALTER TABLE audit_events
    DROP CONSTRAINT IF EXISTS audit_events_action_check;

ALTER TABLE audit_events
    ADD CONSTRAINT audit_events_action_check CHECK (action IN (
        'ai.assist.design',
        'ai.assist.engineering',
        'ai.assist.manufacturing',
        'ai.assist.memory.purged',
        'ai.assist.pricing',
        'ai.assist.proposal',
        'ai.rag.query',
        'apikey.created',
        'apikey.revoked',
        'auth.login',
        'auth.login_denied',
        'auth.logout',
        'auth.register',
        'auth.sso.login',
        'authz.denied',
        'comment.created',
        'comment.deleted',
        'config.approved',
        'config.restored',
        'export.generated',
        'integration.endpoints_issued',
        'key.issued',
        'key.revoked',
        'order.status_changed',
        'payment.refund',
        'project.calculated',
        'project.comment',
        'project.created',
        'project.deleted',
        'project.member_added',
        'project.member_removed',
        'project.member_role_changed',
        'project.review.approved',
        'project.review.requested',
        'project.transferred',
        'review.changes_requested',
        'review.signed',
        'settings.updated',
        'sso.linked',
        'sso.login',
        'sso.login_denied',
        'stair.calculated',
        'stair.config_changed',
        'stair.live_suggestion_applied',
        'stair.live_variation_applied',
        'stair.suggestion_applied',
        'stair.variation_applied',
        'testimonial.created',
        'testimonial.deleted',
        'testimonial.updated',
        'user.deleted',
        'user.grants_changed',
        'user.role_changed',
        'user.status_changed'
    ));