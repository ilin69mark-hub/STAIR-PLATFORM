-- STAIR PLATFORM — Phase C (C4): Review.
-- EDR-0010: ревью проекта — переходы статуса (request/sign-off/request
-- changes) с историей в project_reviews.
--
-- Статусы: draft → in_review → approved; in_review → changes_requested.
-- Каждый переход атомарно обновляет projects.status и добавляет строку
-- project_reviews (аудит). Скоуп по tenant через проекты (SEC-0005):
-- доступ проверяется по членству (EDR-0008).

-- Фиксируем допустимые статусы проекта.
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_status_check;
ALTER TABLE projects ADD CONSTRAINT projects_status_check
    CHECK (status IN ('draft', 'in_review', 'approved', 'changes_requested'));

-- История ревью: строка на переход. ReviewerID/DecidedAt пустые до
-- решения (sign-off / request changes).
CREATE TABLE project_reviews (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    requester_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reviewer_id  UUID REFERENCES users(id) ON DELETE CASCADE,
    decision     TEXT NOT NULL CHECK (decision IN ('requested', 'approved', 'changes_requested')),
    comment      TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at   TIMESTAMPTZ
);

CREATE INDEX project_reviews_project_idx ON project_reviews (project_id, created_at);