-- SEC-002 (forensic 2026-09-26): подделка записей журнала аудита.
--
-- Проблема: колонка audit_events.action была TEXT NOT NULL без CHECK, а
-- POST /api/v1/audit принимал action из тела запроса без allowlist. Любой
-- аутентифицированный пользователь мог записать в журнал безопасности строку
-- с произвольным действием ("user.role_changed", "payment.refunded") и
-- жёстко заданным result="ok" — то есть подделать доказательство (SEC-0013).
-- Воспроизведено до фикса: 201 Created и строка в БД с action
-- "user.role_changed", actor — обычный пользователь.
--
-- Что меняется:
--   1. CHECK на result: клиент не выбирает исход, иначе подделка выглядела бы
--      как отказ и маскировала реальные отказы под "успех".
--   2. CHECK на action: полный реестр известных действий (40 значений,
--      синхронизирован с audit.AllActions() — сверяется тестом
--      TestSEC002_MigrationAllowlistMatchesRegistry; при добавлении нового
--      действия тест валит сборку, пока миграция не обновлена).
--   3. Границы длины текстовых полей, приходящих из тела запроса
--      (совпадают с константами в internal/application/audit/entity.go).
--
-- Отдельно (не в этой миграции): клиентский allowlist реализован в
-- audit.ClientActions() и проверяется в handleRecordAudit — известное, но
-- серверное действие даёт 403 action_not_recordable, неизвестное — 422.

-- 1) CHECK на result.
ALTER TABLE audit_events
    DROP CONSTRAINT IF EXISTS audit_events_result_check;
ALTER TABLE audit_events
    ADD CONSTRAINT audit_events_result_check
    CHECK (result IN ('ok', 'denied', 'failed'));

-- 2) CHECK на action: полный реестр известных действий.
ALTER TABLE audit_events
    DROP CONSTRAINT IF EXISTS audit_events_action_check;
ALTER TABLE audit_events
    ADD CONSTRAINT audit_events_action_check CHECK (action IN (
        -- auth
        'auth.login',
        'auth.login_denied',
        'auth.logout',
        'auth.register',
        -- authz/user
        'api_key.created',
        'api_key.revoked',
        'authz.denied',
        'data.exported',
        'settings.updated',
        'user.role_changed',
        'user.status_changed',
        -- sso
        'sso.linked',
        'sso.login',
        'sso.login_denied',
        -- projects
        'member.added',
        'member.removed',
        'member.role_changed',
        'project.created',
        'project.modified',
        -- config
        'config.approved',
        'config.restored',
        'review.changes',
        'review.requested',
        'review.signed',
        -- ai
        'ai.assist.design',
        'ai.assist.engineering',
        'ai.assist.manufacturing',
        'ai.assist.memory.purged',
        'ai.assist.pricing',
        -- stair
        'stair.calculated',
        'stair.config_changed',
        'stair.live_suggestion_applied',
        'stair.live_variation_applied',
        'stair.suggestion_applied',
        'stair.variation_applied',
        -- commerce
        'order.status_changed',
        'payment.refunded',
        -- testimonial
        'testimonial.created',
        'testimonial.deleted',
        'testimonial.updated'
    ));

-- 3) Границы длины текстовых полей, приходящих из тела запроса.
ALTER TABLE audit_events
    DROP CONSTRAINT IF EXISTS audit_events_len_check;
ALTER TABLE audit_events
    ADD CONSTRAINT audit_events_len_check CHECK (
        length(resource_type) <= 64
        AND length(resource_id)   <= 128
        AND length(detail)        <= 4096
    );
