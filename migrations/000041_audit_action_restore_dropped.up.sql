-- 000041_audit_action_restore_dropped — возврат audit-actions, выпавших из
-- CHECK при переписывании перечня в миграции 000038.
--
-- ЧТО СЛОМАЛОСЬ. 000038 задумывалась как «добавить user.deleted», но вместо
-- точечной правки подменила перечень целиком на другой словарь. Из 41
-- значения, разрешённых 000037, десять исчезли:
--
--     api_key.created      apikey.created       (переименование без правки кода)
--     api_key.revoked      apikey.revoked
--     data.exported        —                    (объявлено, но не пишется)
--     member.added         project.member_added
--     member.removed       project.member_removed
--     member.role_changed  project.member_role_changed
--     payment.refunded     payment.refund
--     project.modified     project.calculated
--     review.changes       review.changes_requested
--     review.requested     project.review.requested
--
-- Девять из них код до сих пор пишет (см. internal/application/audit/entity.go
-- и вызовы record). Каждая такая запись падала по CHECK, а
-- Service.record ошибку глотает — только лог. Итог: журнал безопасности МОЛЧА
-- терял события «конфигурация рассчитана», «CAD выгружен», «конфигурация
-- оптимизирована», «член добавлен/удалён/роль изменена», «запрос на
-- ревью», «смена состояния оплаты», «выдача/отзыв API-ключа».
--
-- ПОЧЕМУ ВОЗВРАЩАЕМ, А НЕ ПЕРЕИМЕНОВЫВАЕМ КОД. Словарь из 000038 не
-- согласован с кодом: он покрывает часть событий, которых код не пишет
-- (project.calculated, export.generated, project.comment), и не покрывает
-- те, которые код пишет. Синхронизация одного словаря — отдельная работа
-- (согласование имён с аналитикой и выгрузками), а терять события аудита
-- нельзя сейчас. Поэтому CHECK становится объединением: оба словаря
-- принимаются, ни одно уже написанное событие не перестаёт попадать в
-- журнал. Лишние значения CHECK безвредны — писать по-прежнему только код.
--
-- Порядок тот же, что в 000037–000039: DROP, затем ADD нового списка.
-- CHECK применяется к существующим строкам, поэтому перед сменой списка
-- стоит убедиться в preflight: go run ./cmd/migrate -preflight.
-- migguard выводит допустимые значения из pg_constraint, то есть проверяет
-- данные против УЖЕ применённого ограничения — этой миграцией он не мешает.

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
        'api_key.created',
        'api_key.revoked',
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
        'data.exported',
        'export.generated',
        'integration.endpoints_issued',
        'key.issued',
        'key.revoked',
        'member.added',
        'member.removed',
        'member.role_changed',
        'order.status_changed',
        'payment.refund',
        'payment.refunded',
        'project.calculated',
        'project.comment',
        'project.created',
        'project.deleted',
        'project.member_added',
        'project.member_removed',
        'project.member_role_changed',
        'project.modified',
        'project.review.approved',
        'project.review.requested',
        'project.transferred',
        'review.changes',
        'review.changes_requested',
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
        'user.deleted',
        'user.grants_changed',
        'user.role_changed',
        'user.status_changed'
    ));
