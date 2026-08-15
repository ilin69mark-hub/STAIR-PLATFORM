-- STAIR PLATFORM — Distributed Processing (EDR-0035, Phase B B4).
-- Асинхронный расчёт: API ставит задание calc.calculate в JobQueue
-- (EDR-0020), воркер выполняет конвейер и фиксирует статус/результат.
--
-- calc_jobs: одна запись = одно фоновое задание. id — идентификатор
-- задания из очереди (job_id, TEXT). payload хранит вход расчёта
-- ({config, options}), result — сериализованный stair.Result при успехе,
-- error — текст при провале. Скоуп tenant'а — как у остальных таблиц.
--
-- Инвариант 4 (EDR-0035): succeeded ⇒ result непустой, failed ⇒ error непустой.

CREATE TABLE calc_jobs (
    id           TEXT PRIMARY KEY,
    tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id      UUID REFERENCES users(id) ON DELETE SET NULL,
    type         TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','running','succeeded','failed')),
    payload      JSONB NOT NULL,
    result       JSONB,
    error        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at   TIMESTAMPTZ,
    finished_at  TIMESTAMPTZ
);

CREATE INDEX calc_jobs_tenant_idx ON calc_jobs (tenant_id);
CREATE INDEX calc_jobs_status_idx ON calc_jobs (status);