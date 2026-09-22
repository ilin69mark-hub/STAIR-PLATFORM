-- STAIR PLATFORM — откат orders kind + nullable.
-- S-136: down обязан переживать данные — consultation-строки (price_json и
-- user_id NULL, разрешены UP-18) несовместимы с возвратом NOT NULL, поэтому
-- удаляются первыми. Консультации — эфемерные лиды; даун в проде только с
-- предварительным дампом (runbook S-129, RPO/RTO S-128).
DELETE FROM orders WHERE kind = 'consultation';

ALTER TABLE orders
    ALTER COLUMN price_json SET NOT NULL;

ALTER TABLE orders
    ALTER COLUMN user_id SET NOT NULL;

ALTER TABLE orders
    DROP COLUMN kind;