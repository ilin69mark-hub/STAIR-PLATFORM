-- 000039_audit_project_transferred — серверное событие «владение проектом
-- передано».
--
-- Зачем: передача проекта — это решение о том, кто теперь отвечает за работу, а
-- не правка роли наблюдателя. В журнале должно быть видно, кто кому отдал
-- проект: это вопрос доверия, и по member.role_changed его не восстановить.
--
-- Тот же приём, что в 000037 и 000038: CHECK из 000029 в уже развёрнутой базе
-- применён, поэтому его обновляет отдельная миграция. Без неё передача
-- падала бы по CHECK — то есть меняла бы владельца без следа в журнале.
--
-- (список действий ниже — копия 000038 плюс project.transferred)
--
-- Зачем: удаление сотрудника (передача его проектов другому человеку и
-- закрытие входа) — это серверное событие безопасности, а не изменение
-- профиля. Показывать его как user.status_changed нельзя: по журналу было бы
-- не видно, что человек именно удалён и куда уехали его проекты.
--
-- Тот же приём, что в 000037: в уже развёрнутой базе CHECK из 000029 уже
-- применён, поэтому его обновляет отдельная миграция, а не перечитывание
-- старой. Без неё удаление падало бы по CHECK — то есть меняло бы доступ,
-- не оставив следа в журнале.

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
