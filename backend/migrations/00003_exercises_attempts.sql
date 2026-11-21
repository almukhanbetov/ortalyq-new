-- +goose Up
CREATE TABLE score_categories (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id   uuid REFERENCES users(id) ON DELETE CASCADE, -- NULL = system category
    name       text NOT NULL CHECK (length(name) BETWEEN 1 AND 60),
    color      text NOT NULL DEFAULT '#1AC6FF',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX score_categories_owner_name_uq ON score_categories (COALESCE(owner_id, '00000000-0000-0000-0000-000000000000'::uuid), lower(name));
INSERT INTO score_categories (id, owner_id, name, color) VALUES
    ('00000000-0000-0000-0000-0000000000c1', NULL, 'Универсальная', '#1AC6FF');

-- An exercise is a stable identity; its content lives in immutable versions.
CREATE TABLE exercises (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_id          uuid NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
    scope              text NOT NULL CHECK (scope IN ('lesson', 'homework')),
    section_id         uuid REFERENCES sections(id) ON DELETE CASCADE,
    kind               text NOT NULL CHECK (kind IN ('test')),
    position           integer NOT NULL DEFAULT 0,
    hidden             boolean NOT NULL DEFAULT false,
    current_version_id uuid,
    copied_from_id     uuid REFERENCES exercises(id) ON DELETE SET NULL,
    created_by         uuid REFERENCES users(id) ON DELETE SET NULL,
    deleted_at         timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CHECK ((scope = 'lesson') = (section_id IS NOT NULL))
);
CREATE INDEX exercises_lesson_idx ON exercises (lesson_id, scope, position) WHERE deleted_at IS NULL;
CREATE INDEX exercises_section_idx ON exercises (section_id, position) WHERE deleted_at IS NULL;

CREATE TABLE exercise_versions (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    exercise_id        uuid NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    version            integer NOT NULL,
    title              text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    check_mode         text NOT NULL DEFAULT 'instant' CHECK (check_mode IN ('instant', 'after')),
    timer_enabled      boolean NOT NULL DEFAULT false,
    timer_seconds      integer CHECK (timer_seconds IS NULL OR timer_seconds BETWEEN 10 AND 36000),
    scoring_enabled    boolean NOT NULL DEFAULT false,
    points_per_correct integer NOT NULL DEFAULT 1 CHECK (points_per_correct BETWEEN 0 AND 100),
    category_id        uuid REFERENCES score_categories(id) ON DELETE SET NULL,
    created_by         uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    UNIQUE (exercise_id, version),
    CHECK (NOT timer_enabled OR timer_seconds IS NOT NULL)
);
ALTER TABLE exercises ADD CONSTRAINT exercises_current_version_fk
    FOREIGN KEY (current_version_id) REFERENCES exercise_versions(id) ON DELETE SET NULL;

CREATE TABLE exercise_questions (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    version_id  uuid NOT NULL REFERENCES exercise_versions(id) ON DELETE CASCADE,
    position    integer NOT NULL,
    prompt      text NOT NULL CHECK (length(prompt) BETWEEN 1 AND 2000),
    multiple    boolean NOT NULL DEFAULT false,
    UNIQUE (version_id, position)
);

CREATE TABLE exercise_options (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id uuid NOT NULL REFERENCES exercise_questions(id) ON DELETE CASCADE,
    position    integer NOT NULL,
    label       text NOT NULL CHECK (length(label) BETWEEN 1 AND 500),
    is_correct  boolean NOT NULL DEFAULT false,
    UNIQUE (question_id, position)
);

-- Attempts belong to a classroom + student and reference the exact version that was taken.
CREATE TABLE attempts (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    classroom_id     uuid NOT NULL REFERENCES classrooms(id) ON DELETE CASCADE,
    exercise_id      uuid NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    version_id       uuid NOT NULL REFERENCES exercise_versions(id) ON DELETE RESTRICT,
    student_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status           text NOT NULL CHECK (status IN ('in_progress', 'finished', 'reset')),
    started_at       timestamptz NOT NULL DEFAULT now(),
    deadline_at      timestamptz,
    finished_at      timestamptz,
    finish_reason    text CHECK (finish_reason IN ('manual', 'timeout', 'homework_submit')),
    correct_count    integer,
    wrong_count      integer,
    unanswered_count integer,
    points_earned    integer,
    points_max       integer,
    reset_at         timestamptz,
    reset_by         uuid REFERENCES users(id) ON DELETE SET NULL,
    updated_at       timestamptz NOT NULL DEFAULT now()
);
-- Only one live (non-reset) attempt per classroom/exercise/student: start is idempotent.
CREATE UNIQUE INDEX attempts_live_uq ON attempts (classroom_id, exercise_id, student_id) WHERE status <> 'reset';
CREATE INDEX attempts_student_idx ON attempts (student_id, classroom_id);
CREATE INDEX attempts_exercise_idx ON attempts (exercise_id);

CREATE TABLE attempt_answers (
    attempt_id          uuid NOT NULL REFERENCES attempts(id) ON DELETE CASCADE,
    question_id         uuid NOT NULL REFERENCES exercise_questions(id) ON DELETE RESTRICT,
    selected_option_ids uuid[] NOT NULL DEFAULT '{}',
    checked             boolean NOT NULL DEFAULT false,
    updated_at          timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (attempt_id, question_id)
);

-- +goose Down
DROP TABLE attempt_answers;
DROP TABLE attempts;
DROP TABLE exercise_options;
DROP TABLE exercise_questions;
ALTER TABLE exercises DROP CONSTRAINT exercises_current_version_fk;
DROP TABLE exercise_versions;
DROP TABLE exercises;
DROP TABLE score_categories;
