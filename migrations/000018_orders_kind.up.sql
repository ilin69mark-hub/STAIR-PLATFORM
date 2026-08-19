-- STAIR PLATFORM — Orders kind + анонимные консультации (store).
-- Разделяет заказ-лид от консультации (запрос обратной связи без расчёта):
--   kind = 'order' — обычный заказ из конструктора (auth, config+price);
--   kind = 'consultation' — анонимная консультация (без user_id, без price).
ALTER TABLE orders
    ADD COLUMN kind TEXT NOT NULL DEFAULT 'order'
        CHECK (kind IN ('order', 'consultation'));

-- Анонимная консультация не требует пользователя.
ALTER TABLE orders
    ALTER COLUMN user_id DROP NOT NULL;

-- У консультации нет снимка цены.
ALTER TABLE orders
    ALTER COLUMN price_json DROP NOT NULL;