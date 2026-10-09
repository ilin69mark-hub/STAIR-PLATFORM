-- 000042_audit_action_restore_return — возврат строк из резерва 000041
-- и удаление резервной таблицы. Закрывает инцидент 2026-10-07 (INFRA-0028 п.2).
--
-- ЧТО СЛУЧИЛОСЬ. Откат 000041 выполняли вручную, по одному оператору, мимо
-- транзакции мигратора: CREATE резервной таблицы, копирование строк и DELETE
-- из журнала зафиксировались, а ADD CONSTRAINT (сужение CHECK) упал. Итог:
-- 8 событий (data.exported, project.modified) остались только в
-- audit_events_action_restore_backup, а журнал их лишился.
--
-- ПОЧЕМУ ВОЗВРАТ БЕЗОПАСЕН. CHECK сейчас широкий (000041 up): все десять
-- возвращённых значений разрешены, вставка проходит. Строки возвращаются с
-- родными id и created_at — это восстановление, а не фабрикация. Дубли
-- исключены предикатом NOT EXISTS (тот же id в журнале = то же событие).
--
-- ПОЧЕМУ МИГРАЦИЯ, А НЕ РУЧНОЙ SQL. Резерв мог остаться и на других стендах
-- (dev-машины, пережившие тот же откат): миграция чинит их тем же кодом.
-- На свежих базах резервной таблицы нет — блок через to_regclass делает
-- миграцию no-op, down/up цикл тестов это проверяет
-- (TestNarrowingRollbackLosesNoRows).
--
-- ПОСЛЕ ЭТОЙ МИГРАЦИИ резервных таблиц audit_events_*_backup в схеме быть не
-- должно. Правило lifecycle резервов: резерв живёт ровно до миграции,
-- которая возвращает строки и удаляет таблицу (INFRA-0028 п.2).

DO $$
BEGIN
    IF to_regclass('public.audit_events_action_restore_backup') IS NOT NULL THEN
        -- Удаляем осиротевшие строки резерва: если down-цепочка ушла ниже
        -- 000041 (например, MigrateToVersion(1) в тестах), родительские
        -- таблицы users/tenants/projects были пересозданы пустыми, и строки
        -- резерва ссылаются на несуществующие FK. В проде это no-op.
        DELETE FROM audit_events_action_restore_backup b
        WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = b.actor_id)
           OR NOT EXISTS (SELECT 1 FROM tenants t WHERE t.id = b.tenant_id)
           OR (b.project_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM projects p WHERE p.id = b.project_id));

        INSERT INTO audit_events
        SELECT * FROM audit_events_action_restore_backup b
        WHERE NOT EXISTS (SELECT 1 FROM audit_events a WHERE a.id = b.id);

        DROP TABLE audit_events_action_restore_backup;
    END IF;
END $$;
