-- S-135: откат conversation memory. Таблица и её индексы удаляются;
-- FK к projects/tenants снимается вместе с таблицей.
DROP TABLE IF EXISTS conversation_messages;