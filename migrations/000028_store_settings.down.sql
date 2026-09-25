-- STAIR PLATFORM — откат 000028 (store_settings + store_rates).
--
-- Откат удаляет введённые цены материалов и настройки магазина: перед этим
-- выгрузите актуальный прайс (GET /api/v1/admin/store/prices, CSV).

DROP TABLE IF EXISTS store_rates;
DROP TABLE IF EXISTS store_settings;
