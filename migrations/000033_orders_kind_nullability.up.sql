-- STAIR PLATFORM — инвариант «вид заказа ↔ заполненность».
--
-- DB-9 (forensic 2026-09-27): миграция 000018 сняла NOT NULL с orders.user_id
-- и orders.price_json, чтобы консультации могли быть анонимными. Но сняла
-- его ДЛЯ ВСЕХ строк и не связала с `kind`. Итог:
--
--   * обычный заказ-лид мог иметь price_json IS NULL или user_id IS NULL;
--   * откат 000018 (удаляет только консультации, затем SET NOT NULL) падал
--     на такой строке: «column price_json contains null values»;
--   * падение внутри миграции помечает версию dirty, после чего `migrate up`
--     не выполняется вовсе — откат блокировал и накат, и подъём приложения
--     (cmd/api вызывает Migrate при старте и завершается с os.Exit(1)).
--
-- То есть одна nullable-колонка без инварианта дала цепочку от неудачного
-- отката до неподнимающегося сервиса.
--
-- Чинение: CHECK связывает `kind` с заполненностью. Консультация по
-- определению анонимна и без цены; обычный заказ — нет.
--
-- Порядок важен: сначала вычищаем строки, нарушающие новый инвариант, и
-- только потом накладываем ограничение. Чистка идёт ДО ALTER TABLE, и если
-- что-то не сошлось — падает здесь, а не на ADD CONSTRAINT: так версия не
-- пачкается (см. DB-001 про dirty-состояние).
--
-- Нарушители помечаются, а не удаляются: заказ — бизнес-сущность, и молча
-- выбросить его из-за одного NULL-поля хуже, чем сделать его явно
-- неполным. Поле "__invalid_kind_nullable" позволяет найти такие строки
-- запросом. Клиентский код при этом отвергнет их как некорректные данные —
-- что верно: такой заказ нельзя ни посчитать, ни отгрузить.
--
-- NOT VALID + VALIDATE (а не сразу ADD): сначала ограничение накладывается
-- без просмотра таблицы (мгновенно), затем валидация отдельным шагом. Для
-- больших таблиц это единственный способ не держать блокировку на всё
-- время скана.

UPDATE orders
   SET price_json = '{"__invalid_kind_nullable":true,"reason":"order row with NULL price_json"}'
 WHERE kind = 'order' AND price_json IS NULL;

-- Для user_id плейсхолдер не годится: FK orders_user_id_fkey требует
-- существующего пользователя (это вскрылось при проверке отката — подстановка
-- нулевого UUID роняла миграцию на FKey). Берём реального пользователя
-- tenant'а; если его нет, заказ не представляем и удаляем, считая явно.
DO $$
DECLARE
    v_tenant uuid;
    v_fallback uuid;
    v_affected int;
BEGIN
    FOR v_tenant IN SELECT DISTINCT tenant_id FROM orders WHERE kind = 'order' AND user_id IS NULL
    LOOP
        SELECT id INTO v_fallback FROM users
         WHERE tenant_id = v_tenant ORDER BY created_at LIMIT 1;

        IF v_fallback IS NULL THEN
            SELECT count(*) INTO v_affected FROM orders
             WHERE kind = 'order' AND user_id IS NULL AND tenant_id = v_tenant;
            RAISE NOTICE
                '000033: удалено % заказов без владельца (tenant %): пользователей нет', v_affected, v_tenant;
            DELETE FROM orders WHERE kind = 'order' AND user_id IS NULL AND tenant_id = v_tenant;
        ELSE
            UPDATE orders SET user_id = v_fallback
             WHERE kind = 'order' AND user_id IS NULL AND tenant_id = v_tenant;
        END IF;
    END LOOP;
END $$;

-- Консультация обязана быть анонимной: если у неё вдруг есть цена или
-- пользователь, это данные другого вида — приводим к корректному виду
-- (обнуляем), а не отвергаем.
UPDATE orders
   SET price_json = NULL, user_id = NULL
 WHERE kind = 'consultation' AND (price_json IS NOT NULL OR user_id IS NOT NULL);

ALTER TABLE orders
    DROP CONSTRAINT IF EXISTS orders_kind_nullability_check;

ALTER TABLE orders
    ADD CONSTRAINT orders_kind_nullability_check
    CHECK (
        (kind = 'consultation' AND price_json IS NULL AND user_id IS NULL)
        OR
        (kind = 'order'      AND price_json IS NOT NULL AND user_id IS NOT NULL)
    ) NOT VALID;

-- Валидация отдельным шагом: проверяет уже наложенное ограничение, читая
-- таблицу, но не мешая записи (обычный ACCESS EXCLUSIVE SHARE).
ALTER TABLE orders VALIDATE CONSTRAINT orders_kind_nullability_check;
