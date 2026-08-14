-- STAIR PLATFORM — Phase C (C4): откат Review.

DROP TABLE IF EXISTS project_reviews;

ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_status_check;