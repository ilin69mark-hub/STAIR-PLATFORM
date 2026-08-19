-- STAIR PLATFORM — откат orders kind + nullable.
ALTER TABLE orders
    ALTER COLUMN price_json SET NOT NULL;

ALTER TABLE orders
    ALTER COLUMN user_id SET NOT NULL;

ALTER TABLE orders
    DROP COLUMN kind;