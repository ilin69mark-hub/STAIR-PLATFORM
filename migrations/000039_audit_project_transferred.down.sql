-- Откат 000039: возвращаем реестр действий аудита к состоянию без
-- project.transferred.
--
-- ВНИМАНИЕ: записи project.transferred остаются в журнале и нарушат CHECK,
-- поэтому их нужно удалить до сужения ограничения (как в 000037 для
-- grants_changed и в 000038 для user.deleted). Откат удаляет историю передач —
-- поэтому применять его стоит только на тестовой базе.

DELETE FROM audit_events WHERE action = 'project.transferred';

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
        'user.grants_changed',
        'user.role_changed',
        'user.status_changed'
    ));
