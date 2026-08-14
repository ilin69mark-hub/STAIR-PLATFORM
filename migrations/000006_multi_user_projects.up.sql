-- STAIR PLATFORM — Phase C (C1): Multi-user Projects.
-- EDR-0008: владелец проекта + члены с ролями (owner/editor/viewer).
--
-- Вводим членство проектов: каждый проект имеет ровно одного владельца
-- (owner) и может иметь членов с ролями editor/viewer. Доступ и список
-- проектов строятся из членства (SEC-0005 скоуп — tenant через проекты).

-- Владелец проекта (nullable для унаследованных записей).
ALTER TABLE projects ADD COLUMN owner_id UUID REFERENCES users(id);

-- Членство проекта: роль-модель (EDR-0008). Владелец хранится в этой
-- таблице как член с ролью 'owner'; owner_id в projects — денормализация
-- для быстрых проверок. Ровно один owner на проект.
CREATE TABLE project_members (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, user_id)
);

-- Ровно один владелец на проект.
CREATE UNIQUE INDEX project_members_one_owner
    ON project_members (project_id) WHERE role = 'owner';

CREATE INDEX project_members_user_idx ON project_members (user_id);
CREATE INDEX projects_owner_idx ON projects (owner_id);