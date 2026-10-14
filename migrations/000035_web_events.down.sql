-- Откат 000035: таблица клиентских событий витрины.
DROP INDEX IF EXISTS web_events_created_idx;
DROP INDEX IF EXISTS web_events_session_created_idx;
DROP INDEX IF EXISTS web_events_name_created_idx;
DROP TABLE IF EXISTS web_events;
