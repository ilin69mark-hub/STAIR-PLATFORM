-- STAIR PLATFORM — Phase C (C3): Comments.
-- EDR-0009: комментарии к проекту — обсуждение в рамках участников.
--
-- Любой член проекта (owner/editor/viewer) может оставить комментарий.
-- Удаление: автор комментария или владелец проекта. Комментарии скоупятся
-- по tenant через проекты (SEC-0005): доступ проверяется по членству.

CREATE TABLE project_comments (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    author_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body       TEXT NOT NULL CHECK (length(body) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX project_comments_project_idx ON project_comments (project_id, created_at);
CREATE INDEX project_comments_author_idx ON project_comments (author_id);