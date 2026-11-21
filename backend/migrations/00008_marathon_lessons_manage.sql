-- +goose Up
-- Marathons, stage 2: managing lessons inside a marathon.
-- * course_id: the marathon's own course in the teacher's materials. Lessons created or copied inside the
--   marathon live there, so a teacher's personal courses are never changed by these actions.
-- * cover_color: the lesson cover shown in this marathon (falls back to the course colour).
-- * removed_at/removed_by: "Удалить" removes a lesson from the marathon only. The lesson template, attempts and
--   progress stay; adding the same lesson again restores it with its progress.

ALTER TABLE marathon_settings ADD COLUMN course_id uuid REFERENCES courses(id) ON DELETE SET NULL;

ALTER TABLE marathon_lessons ADD COLUMN cover_color text CHECK (cover_color ~ '^#[0-9A-Fa-f]{6}$');
ALTER TABLE marathon_lessons ADD COLUMN removed_at timestamptz;
ALTER TABLE marathon_lessons ADD COLUMN removed_by uuid REFERENCES users(id) ON DELETE SET NULL;

DROP INDEX marathon_lessons_order_idx;
CREATE INDEX marathon_lessons_order_idx ON marathon_lessons (classroom_id, position) WHERE removed_at IS NULL;

-- +goose Down
-- Before this migration a removed lesson did not exist, so removed rows are dropped (with their progress rows).
DELETE FROM marathon_lessons WHERE removed_at IS NOT NULL;
DROP INDEX marathon_lessons_order_idx;
CREATE INDEX marathon_lessons_order_idx ON marathon_lessons (classroom_id, position);
ALTER TABLE marathon_lessons DROP COLUMN removed_by;
ALTER TABLE marathon_lessons DROP COLUMN removed_at;
ALTER TABLE marathon_lessons DROP COLUMN cover_color;
ALTER TABLE marathon_settings DROP COLUMN course_id;
