-- +goose Up
-- Marathons, stage 5: passage modes "open", "progressive" (without teacher check) and "closed",
-- the lesson opening date ("Дата открытия") and mode change through the progress epoch.
-- * progress_epoch: lesson statuses count only in the current epoch. A mode change starts a new epoch:
--   answers, attempts and points stay, every lesson becomes "not completed" (as the original warns).
--   Rows of earlier epochs are kept for history.
-- * marathon_lessons.open_at: the lesson opens to students at this moment (open mode).
-- * marathon_lesson_unlocks: closed mode, the teacher opened a lesson for everyone (student_id NULL)
--   or for one student, in a given epoch.
-- * marathon_mode_changes: who changed the mode and when.
-- "interval" and "weekdays" stay in the CHECK of 00006 but are not accepted by the API yet (open question Н1).

ALTER TABLE marathon_settings ADD COLUMN progress_epoch integer NOT NULL DEFAULT 1 CHECK (progress_epoch >= 1);
ALTER TABLE marathon_lessons ADD COLUMN open_at timestamptz;

CREATE TABLE marathon_lesson_unlocks (
    id           bigserial   PRIMARY KEY,
    classroom_id uuid        NOT NULL,
    lesson_id    uuid        NOT NULL,
    student_id   uuid        REFERENCES users(id) ON DELETE CASCADE, -- NULL = for every student
    epoch        integer     NOT NULL,
    unlocked_by  uuid        REFERENCES users(id) ON DELETE SET NULL,
    unlocked_at  timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (classroom_id, lesson_id) REFERENCES marathon_lessons (classroom_id, lesson_id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX marathon_lesson_unlocks_uq ON marathon_lesson_unlocks
    (classroom_id, lesson_id, COALESCE(student_id, '00000000-0000-0000-0000-000000000000'::uuid), epoch);

CREATE TABLE marathon_mode_changes (
    id           bigserial   PRIMARY KEY,
    classroom_id uuid        NOT NULL REFERENCES classrooms(id) ON DELETE CASCADE,
    from_mode    text        NOT NULL,
    to_mode      text        NOT NULL,
    from_epoch   integer     NOT NULL,
    to_epoch     integer     NOT NULL,
    changed_by   uuid        REFERENCES users(id) ON DELETE SET NULL,
    changed_at   timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
-- Statuses of later epochs would count again as epoch 1 is the only one before this migration.
DELETE FROM marathon_lesson_progress p USING marathon_settings ms
    WHERE ms.classroom_id = p.classroom_id AND p.epoch <> ms.progress_epoch;
UPDATE marathon_lesson_progress SET epoch = 1;
UPDATE marathon_settings SET mode = 'open';
DROP TABLE marathon_mode_changes;
DROP TABLE marathon_lesson_unlocks;
ALTER TABLE marathon_lessons DROP COLUMN open_at;
ALTER TABLE marathon_settings DROP COLUMN progress_epoch;
