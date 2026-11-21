-- +goose Up
CREATE TABLE homeworks (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    classroom_id uuid NOT NULL REFERENCES classrooms(id) ON DELETE CASCADE,
    lesson_id    uuid NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
    student_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status       text NOT NULL DEFAULT 'not_submitted'
                 CHECK (status IN ('not_submitted', 'pending', 'revision', 'done')),
    status_changed_at timestamptz NOT NULL DEFAULT now(),
    status_changed_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (classroom_id, lesson_id, student_id)
);
CREATE INDEX homeworks_classroom_status_idx ON homeworks (classroom_id, status);

-- Each submission is an immutable snapshot of answers.
CREATE TABLE homework_submissions (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    homework_id  uuid NOT NULL REFERENCES homeworks(id) ON DELETE CASCADE,
    number       integer NOT NULL,
    submitted_by uuid REFERENCES users(id) ON DELETE SET NULL,
    submitted_at timestamptz NOT NULL DEFAULT now(),
    snapshot     jsonb NOT NULL,
    UNIQUE (homework_id, number)
);

CREATE TABLE homework_status_events (
    id          bigserial PRIMARY KEY,
    homework_id uuid NOT NULL REFERENCES homeworks(id) ON DELETE CASCADE,
    from_status text NOT NULL,
    to_status   text NOT NULL,
    actor_id    uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX homework_status_events_idx ON homework_status_events (homework_id, created_at);

-- +goose Down
DROP TABLE homework_status_events;
DROP TABLE homework_submissions;
DROP TABLE homeworks;
