-- 000037_audit_grants_action — серверное событие «изменены права сотрудника».
--
-- Зачем: EDR-0041 ввёл действие user.grants_changed, но реестр допустимых
-- значений audit_events.action живёт в CHECK из миграции 000029. В свежей
-- базе он обновляется там же (файл правится вручную, и его сверяет
-- TestSEC002_MigrationAllowlistMatchesRegistry). А вот в уже развёрнутой базе
-- миграция 000029 применена, номер версии записан, и перечитать её нельзя —
-- поэтому нужен отдельный шаг.
--
-- Без этого выдача прав падала бы по CHECK, то есть меняла бы доступ, не
-- оставляя следа в журнале безопасности.
--
-- Список взят из действующего CHECK в базе, а не перечислен руками: в
-- предыдущей версии этой миграции часть действий потерялась бы молча.

ALTER TABLE audit_events
    DROP CONSTRAINT IF EXISTS audit_events_action_check;

ALTER TABLE audit_events
    ADD CONSTRAINT audit_events_action_check CHECK (action IN (
        'ai.assist.design',
        'ai.assist.engineering',
        'ai.assist.manufacturing',
        'ai.assist.memory.purged',
        'ai.assist.pricing',
        'api_key.created',
        'api_key.revoked',
        'auth.login',
        'auth.login_denied',
        'auth.logout',
        'auth.register',
        'authz.denied',
        'config.approved',
        'config.restored',
        'data.exported',
        'member.added',
        'member.removed',
        'member.role_changed',
        'order.status_changed',
        'payment.refunded',
        'project.created',
        'project.modified',
        'review.changes',
        'review.requested',
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
