-- STAIR PLATFORM — MVP-08: единый канонический снапшот расчёта.
--
-- Оригинальная схема хранила отдельные JSONB-представления этапов
-- (flight, geometry, manufacturing, pricing). Снапшот конвейера уже
-- содержит все части как единый экспортный документ (REP-0001):
-- дублирование колонок избыточно и создаёт риск рассинхронизации.
-- Удаляем дублирующие колонки; result остаётся единственным источником.

ALTER TABLE calculations
    DROP COLUMN flight,
    DROP COLUMN geometry,
    DROP COLUMN manufacturing,
    DROP COLUMN pricing;