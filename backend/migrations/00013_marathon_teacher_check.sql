-- +goose Up
-- Marathons, stage 6: "Проверка учителем".
-- * marathon_settings.teacher_check: lessons are sent to the teacher instead of being completed by the student.
-- * marathon_settings.unlock_after: "Открытие уроков" of the progressive mode while the check is on —
--   after the teacher accepts the previous lesson (teacher_check) or right after it is sent (submission).
--   Ignored in the other modes and while the check is off.
-- * marathon_lesson_progress gets the review statuses of the current epoch:
--     submitted — sent to the teacher, answers locked;
--     revision  — returned by the teacher, wrong instantly-checked answers editable again (as homework);
--     completed — completed by the student (check off) or accepted by the teacher (accepted = true,
--                 answers stay locked, rating and comment for the student).
--   "Отменить завершение" moves an accepted lesson back to submitted; rating and comment stay in the row as
--   the previous review (shown to the student only while the lesson is completed).
-- * marathon_lesson_submissions: an immutable snapshot of answers per submission.
-- * marathon_lesson_events: who changed which status and when (audit, as homework_status_events).

ALTER TABLE marathon_settings ADD COLUMN teacher_check boolean NOT NULL DEFAULT false;
ALTER TABLE marathon_settings ADD COLUMN unlock_after text NOT NULL DEFAULT 'teacher_check'
    CHECK (unlock_after IN ('teacher_check', 'submission'));

ALTER TABLE marathon_lesson_progress DROP CONSTRAINT marathon_lesson_progress_status_check;
ALTER TABLE marathon_lesson_progress ADD CONSTRAINT marathon_lesson_progress_status_check
    CHECK (status IN ('submitted', 'revision', 'completed'));
ALTER TABLE marathon_lesson_progress ALTER COLUMN completed_at DROP NOT NULL;
ALTER TABLE marathon_lesson_progress ALTER COLUMN completed_at DROP DEFAULT;
ALTER TABLE marathon_lesson_progress ADD COLUMN submitted_at timestamptz;
ALTER TABLE marathon_lesson_progress ADD COLUMN accepted boolean NOT NULL DEFAULT false;
ALTER TABLE marathon_lesson_progress ADD COLUMN completed_by uuid REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE marathon_lesson_progress ADD COLUMN reviewed_at timestamptz;
ALTER TABLE marathon_lesson_progress ADD COLUMN reviewed_by uuid REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE marathon_lesson_progress ADD COLUMN rating text CHECK (rating IN ('not_bad', 'nice', 'amazing'));
ALTER TABLE marathon_lesson_progress ADD COLUMN teacher_comment text CHECK (char_length(teacher_comment) <= 2000);
ALTER TABLE marathon_lesson_progress ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
-- Rows before this migration are lessons the student completed with «Завершить урок».
UPDATE marathon_lesson_progress SET completed_by = student_id WHERE completed_by IS NULL;
ALTER TABLE marathon_lesson_progress ADD CONSTRAINT marathon_lesson_progress_completed_check
    CHECK ((status = 'completed') = (completed_at IS NOT NULL));
ALTER TABLE marathon_lesson_progress ADD CONSTRAINT marathon_lesson_progress_accepted_check
    CHECK (NOT accepted OR status = 'completed');
CREATE INDEX marathon_lesson_progress_pending_idx ON marathon_lesson_progress (classroom_id, epoch) WHERE status = 'submitted';

CREATE TABLE marathon_lesson_submissions (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    classroom_id uuid        NOT NULL,
    lesson_id    uuid        NOT NULL,
    student_id   uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    epoch        integer     NOT NULL,
    number       integer     NOT NULL,
    submitted_at timestamptz NOT NULL DEFAULT now(),
    snapshot     jsonb       NOT NULL,
    UNIQUE (classroom_id, lesson_id, student_id, epoch, number),
    FOREIGN KEY (classroom_id, lesson_id) REFERENCES marathon_lessons (classroom_id, lesson_id) ON DELETE CASCADE
);

CREATE TABLE marathon_lesson_events (
    id           bigserial   PRIMARY KEY,
    classroom_id uuid        NOT NULL,
    lesson_id    uuid        NOT NULL,
    student_id   uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    epoch        integer     NOT NULL,
    from_status  text        NOT NULL CHECK (from_status IN ('none', 'submitted', 'revision', 'completed')),
    to_status    text        NOT NULL CHECK (to_status IN ('submitted', 'revision', 'completed')),
    rating       text,
    actor_id     uuid        REFERENCES users(id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (classroom_id, lesson_id) REFERENCES marathon_lessons (classroom_id, lesson_id) ON DELETE CASCADE
);
CREATE INDEX marathon_lesson_events_idx ON marathon_lesson_events (classroom_id, lesson_id, student_id, created_at);

-- +goose Down
-- Before this migration only completed lessons existed: other statuses are dropped, completions are kept.
DROP TABLE marathon_lesson_events;
DROP TABLE marathon_lesson_submissions;
DELETE FROM marathon_lesson_progress WHERE status <> 'completed';
DROP INDEX marathon_lesson_progress_pending_idx;
ALTER TABLE marathon_lesson_progress DROP CONSTRAINT marathon_lesson_progress_accepted_check;
ALTER TABLE marathon_lesson_progress DROP CONSTRAINT marathon_lesson_progress_completed_check;
ALTER TABLE marathon_lesson_progress DROP COLUMN updated_at;
ALTER TABLE marathon_lesson_progress DROP COLUMN teacher_comment;
ALTER TABLE marathon_lesson_progress DROP COLUMN rating;
ALTER TABLE marathon_lesson_progress DROP COLUMN reviewed_by;
ALTER TABLE marathon_lesson_progress DROP COLUMN reviewed_at;
ALTER TABLE marathon_lesson_progress DROP COLUMN completed_by;
ALTER TABLE marathon_lesson_progress DROP COLUMN accepted;
ALTER TABLE marathon_lesson_progress DROP COLUMN submitted_at;
ALTER TABLE marathon_lesson_progress ALTER COLUMN completed_at SET DEFAULT now();
ALTER TABLE marathon_lesson_progress ALTER COLUMN completed_at SET NOT NULL;
ALTER TABLE marathon_lesson_progress DROP CONSTRAINT marathon_lesson_progress_status_check;
ALTER TABLE marathon_lesson_progress ADD CONSTRAINT marathon_lesson_progress_status_check CHECK (status IN ('completed'));
ALTER TABLE marathon_settings DROP COLUMN unlock_after;
ALTER TABLE marathon_settings DROP COLUMN teacher_check;
