-- STAIR PLATFORM — откат orders kind + nullable.
--
-- S-136: down обязан переживать данные.
--
-- DB-9 (forensic 2026-09-27): прежний откат удалял ТОЛЬКО consultation-строки
-- (`DELETE FROM orders WHERE kind = 'consultation'`), после чего делал
-- `ALTER COLUMN price_json SET NOT NULL` и `user_id SET NOT NULL`. Но UP-18
-- снимает NOT NULL с этих колонок ДЛЯ ВСЕХ заказов, а не только для
-- консультаций, и никакого CHECK не связывает `kind` с их заполненностью.
-- Поэтому обычный заказ-лид с `price_json IS NULL` или `user_id IS NULL`
-- проходил, а откат на нём падал:
--
--   ERROR: column "price_json" of relation "orders" contains null values
--
-- Падение внутри миграции помечает версию dirty, и после этого `migrate up`
-- не выполняется ВООБЩЕ — то есть неудачный откат блокировал и накат, и
-- подъём приложения (cmd/api вызывает Migrate при старте).
--
-- Что здесь происходит: удаляются все строки, несовместимые со СХЕМОЙ,
-- возвращаемой откатом. Для консультаций это оправданно (э��емерные лиды).
-- Для обычных заказов удаление — потеря данных, поэтому такие строки
-- ЗАПОЛНЯЮТСЯ заглушками: откат уже необратим, а лишить заказ цены или
-- владельца молча хуже, чем сделать его неполным явно. Значения
-- не соответствуют домену (`Contact`, `PriceJSON` требуют значений), поэтому
-- после отката их обязаны разобрать вручную — это видно по формату строки.
--
-- Правильность ситуации предотвращает миграция 000033: CHECK связывает `kind`
-- с заполненностью, так что обычный заказ с NULL price_json больше не
-- возникает, и в будущем откату не придётся заполнять заглушки.
--
-- Даун в проде — только с предварительным дампом (runbook S-129, RPO/RTO S-128).

-- Консультации: анонимные лиды без пользователя и цены.
DELETE FROM orders WHERE kind = 'consultation';

-- Обычные заказы, у которых UP-18 разрешил пустые значения. Помечаем
-- невалидное состояние, чтобы после отката его нельзя было принять за
-- рабочие данные.
UPDATE orders
   SET price_json = '{"__rolled_back_000018":true,"reason":"price_json was NULL before rollback"}'
 WHERE kind = 'order' AND price_json IS NULL;

-- Обычные заказы, у которых UP-18 разрешил пустые значения.
--
-- Для user_id плейсхолдер вида '00000000-...' не годится: стоит FK
-- orders_user_id_fkey → users(id), и подстановка несуществующего UUID роняла
-- бы откат на FKey (вскрыто при проверке на данных). Поэтому берём РЕАЛЬНОГО
-- пользователя того же tenant'а; если его нет — заказ не представляем (ни
-- владельца, ни цены) и удаляем, считая явно.
DO $$
DECLARE
    v_tenant   uuid;
    v_fallback uuid;
    v_affected int;
BEGIN
    FOR v_tenant IN
        SELECT DISTINCT tenant_id FROM orders WHERE kind = 'order' AND user_id IS NULL
    LOOP
        SELECT id INTO v_fallback FROM users
         WHERE tenant_id = v_tenant ORDER BY created_at LIMIT 1;

        IF v_fallback IS NULL THEN
            SELECT count(*) INTO v_affected FROM orders
             WHERE kind = 'order' AND user_id IS NULL AND tenant_id = v_tenant;
            RAISE NOTICE
                'rollback 000018: удалено % заказов tenant % — нет пользователя-владельца',
                v_affected, v_tenant;
            DELETE FROM orders
             WHERE kind = 'order' AND user_id IS NULL AND tenant_id = v_tenant;
        ELSE
            UPDATE orders SET user_id = v_fallback
             WHERE kind = 'order' AND user_id IS NULL AND tenant_id = v_tenant;
        END IF;
    END LOOP;
END $$;

-- Контроль: если что-то осталось несовместимым, падаем ДО изменения схемы.
-- Иначе ALTER упадёт на середине и оставит версию dirty.
DO $$
DECLARE bad int;
BEGIN
    SELECT count(*) INTO bad FROM orders
     WHERE price_json IS NULL OR user_id IS NULL;
    IF bad > 0 THEN
        RAISE EXCEPTION
            'rollback 000018: % rows still have NULL price_json/user_id', bad;
    END IF;
END $$;

ALTER TABLE orders
    ALTER COLUMN price_json SET NOT NULL;

ALTER TABLE orders
    ALTER COLUMN user_id SET NOT NULL;

ALTER TABLE orders
    DROP COLUMN kind;
