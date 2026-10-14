-- STAIR PLATFORM — откат 000027 (tier_id в платёжных интентах).
--
-- Данные об оплаченных услугах теряются: перед откатом убедитесь, что отчётность
-- по услугам уже выгружена.

DROP INDEX IF EXISTS payment_intents_user_idx;

ALTER TABLE payment_intents
    DROP COLUMN IF EXISTS tier_id;
