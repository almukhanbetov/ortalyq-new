-- +goose Up
-- Marathons, stage 8: the teacher's lesson preview («Режим предпросмотра»).
-- The owner of a lesson takes its tests as a student would, with the timer, instant checks and results, but in
-- separate tables: there is no classroom_id, so preview rows can never reach attempts, statistics, the rating,
-- review counters, lesson progress or the opening of the next lesson. Rows belong to the teacher and the
-- exercise version they started on. «Сбросить» archives a row (status reset), like a student attempt reset.
CREATE TABLE preview_attempts (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    exercise_id      uuid        NOT NULL REFERENCES exercises(id) ON DELETE CASCADE,
    version_id       uuid        NOT NULL REFERENCES exercise_versions(id) ON DELETE CASCADE,
    teacher_id       uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status           text        NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'finished', 'reset')),
    started_at       timestamptz NOT NULL DEFAULT now(),
    deadline_at      timestamptz,
    finished_at      timestamptz,
    finish_reason    text CHECK (finish_reason IN ('manual', 'timeout')),
    correct_count    integer,
    wrong_count      integer,
    unanswered_count integer,
    points_earned    integer,
    points_max       integer,
    reset_at         timestamptz,
    updated_at       timestamptz NOT NULL DEFAULT now()
);
-- One live preview attempt per teacher and exercise (a repeated start returns it).
CREATE UNIQUE INDEX preview_attempts_live_uq ON preview_attempts (exercise_id, teacher_id) WHERE status <> 'reset';

CREATE TABLE preview_answers (
    attempt_id          uuid        NOT NULL REFERENCES preview_attempts(id) ON DELETE CASCADE,
    question_id         uuid        NOT NULL REFERENCES exercise_questions(id) ON DELETE CASCADE,
    selected_option_ids uuid[]      NOT NULL DEFAULT '{}',
    checked             boolean     NOT NULL DEFAULT false,
    updated_at          timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (attempt_id, question_id)
);

-- +goose Down
DROP TABLE preview_answers;
DROP TABLE preview_attempts;
