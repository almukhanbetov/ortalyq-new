-- +goose Up
-- Marathons, stages 2–4 (minimal slice): end date, lessons, enrolled students and per-student lesson progress.
-- Finishing a marathon never deletes or resets anything: it only closes access after end_at (compared with now() of the DB).

-- "Дата завершения": after this moment students may no longer start or continue lessons.
ALTER TABLE marathon_settings ADD COLUMN end_at timestamptz;

-- Lessons of a marathon in their order. Each lesson also has a classroom_lessons row, so the shared
-- RequireLesson check, lessons_count and attempts keep working unchanged.
CREATE TABLE marathon_lessons (
    classroom_id uuid        NOT NULL REFERENCES classrooms(id) ON DELETE CASCADE,
    lesson_id    uuid        NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
    position     integer     NOT NULL,
    added_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (classroom_id, lesson_id)
);
CREATE INDEX marathon_lessons_order_idx ON marathon_lessons (classroom_id, position);

-- Students of a marathon. A classroom_members row (role student) is created in the same transaction.
CREATE TABLE marathon_enrollments (
    classroom_id uuid        NOT NULL REFERENCES classrooms(id) ON DELETE CASCADE,
    student_id   uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    activated_at timestamptz NOT NULL DEFAULT now(),
    added_by     uuid        REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (classroom_id, student_id)
);

-- A lesson the student has completed. epoch is reserved for the future "change mode resets progress" rule.
CREATE TABLE marathon_lesson_progress (
    classroom_id uuid        NOT NULL,
    lesson_id    uuid        NOT NULL,
    student_id   uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    epoch        integer     NOT NULL DEFAULT 1,
    status       text        NOT NULL DEFAULT 'completed' CHECK (status IN ('completed')),
    completed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (classroom_id, lesson_id, student_id, epoch),
    FOREIGN KEY (classroom_id, lesson_id) REFERENCES marathon_lessons (classroom_id, lesson_id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE marathon_lesson_progress;
DROP TABLE marathon_enrollments;
DROP TABLE marathon_lessons;
ALTER TABLE marathon_settings DROP COLUMN end_at;
