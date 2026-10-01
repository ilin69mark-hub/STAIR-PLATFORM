-- STAIR PLATFORM — Payments: оплата услуг с витрины без проекта (этап 4).
--
-- До этого checkout был только для проекта инженерной панели. Публичный сайт
-- продаёт фиксированные услуги (каталог тарифов), и платёж к проекту не
-- привязан: project_id остаётся NULL, а купленное хранится в tier_id.
--
-- tier_id — код услуги из серверного каталога (S-150): цена определяется по
-- нему, из тела запроса сумма не принимается. NULL у старых платежей за
-- проекты — это нормально и означает «оплата проекта».

ALTER TABLE payment_intents
    ADD COLUMN tier_id TEXT;

COMMENT ON COLUMN payment_intents.tier_id IS
    'Код услуги из серверного каталога (S-150); NULL — оплата проекта';

CREATE INDEX payment_intents_user_idx
    ON payment_intents (user_id, created_at DESC)
    WHERE user_id IS NOT NULL;
