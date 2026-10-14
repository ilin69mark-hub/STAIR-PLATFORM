-- 000035_web_events — приём клиентских событий витрины (воронка).
--
-- Зачем: в БД нет ничего о поведении посетителя до заявки. Известно, что
-- человек дошёл до расчёта, но не известно, ГДЕ он застрял и на каком шаге
-- бросил. Именно это и нужно владельцу: где идёт трафик, где трудности,
-- где отток.
--
-- Принципы, из которых таблица и вытекает:
--   1. Персональных данных нет. IP не сохраняется, User-Agent не сохраняется:
--      visitor — HMAC(соль сервера, ip+ua), обрезанный до 16 hex-символов. Без
--      соли (STAIR_ANALYTICS_SALT пуст) visitor = '' и остаются только
--      session_id — то есть пересечь визиты нельзя вообще. Это осознанный
--      компромисс: «уникальные посетители» перестают быть точными, зато
--      отключить идентификацию можно одной переменной окружения.
--   2. Согласие — часть записи. consent_version доказывает, что событие
--      пришло после согласия; без согласия клиент ничего не отправляет, а
--      сервер принимает только версию из действующей политики.
--   3. Событие не содержит произвольного текста. name — из каталога
--      (application/funnel.AllEvents, сверяется тестом), props — только
--      скаляры, ключ ≤ 32 символов, значение ≤ 64. Ограничение на длину
--      значения не паранойя: без него в props могло бы утечь то, что человек
--      ввёл в поле формы.
--   4. Порог (bucket) по часу для отчётов: отчёт «воронка» строится
--      предложениями GROUP BY, отдельная таблица бакетов не нужна, пока
--      не понадобится тяжёлая агрегация за годы.

CREATE TABLE IF NOT EXISTS web_events (
    id              BIGSERIAL PRIMARY KEY,
    session_id      UUID        NOT NULL,
    name            TEXT        NOT NULL,
    props           JSONB       NOT NULL DEFAULT '{}'::jsonb,
    path            TEXT        NOT NULL DEFAULT '',
    visitor         TEXT        NOT NULL DEFAULT '',
    browser         TEXT        NOT NULL DEFAULT '',
    consent_version INT         NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Имя события: только каталог, латиница в нижнем регистре с точками.
    -- Список значений держит application/funnel.AllEvents (тест
    -- TestMigrationEventNamesMatchCatalog валит сборку при расхождении).
    CONSTRAINT web_events_name_check
        CHECK (name ~ '^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*){0,3}$'),
    -- props обязаны быть объектом: массив или скаляр здесь невалидны.
    CONSTRAINT web_events_props_check
        CHECK (jsonb_typeof(props) = 'object'),
    -- Границы текста приходят из тела запроса (см. funnel.MaxPropValueLen).
    CONSTRAINT web_events_path_check   CHECK (length(path) <= 128),
    CONSTRAINT web_events_visitor_check CHECK (length(visitor) <= 32),
    CONSTRAINT web_events_browser_check CHECK (length(browser) <= 32),
    CONSTRAINT web_events_consent_check CHECK (consent_version > 0)
);

-- Отчёт всегда смотрит в окно «последние N дней» и группирует по имени —
-- этот индекс покрывает и воронку, и «где бросают».
CREATE INDEX IF NOT EXISTS web_events_name_created_idx
    ON web_events (name, created_at DESC);
-- Разбор конкретного визита: путь конкретного пользователя при разборе
-- «почему бросил».
CREATE INDEX IF NOT EXISTS web_events_session_created_idx
    ON web_events (session_id, created_at);
-- Чистка по возрасту (worker, sys.cleanup_web_events).
CREATE INDEX IF NOT EXISTS web_events_created_idx
    ON web_events (created_at);

COMMENT ON TABLE web_events IS
    'События витрины для воронки. Пишутся ТОЛЬКО после согласия посетителя. IP и User-Agent не хранятся; visitor — HMAC от них с солью сервера.';
COMMENT ON COLUMN web_events.visitor IS
    'HMAC-SHA256(STAIR_ANALYTICS_SALT, ip + ua)[:16]. Пустая строка, если соль не задана: пересечь визиты невозможно.';
COMMENT ON COLUMN web_events.consent_version IS
    'Версия политики cookie, под которой посетитель дал согласие. Доказательство того, что запись допустима.';
