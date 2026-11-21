-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text NOT NULL,
    password_hash text NOT NULL,
    full_name     text NOT NULL,
    role          text NOT NULL CHECK (role IN ('admin', 'teacher', 'student')),
    locale        text NOT NULL DEFAULT 'ru' CHECK (locale IN ('ru', 'kk', 'en')),
    timezone      text NOT NULL DEFAULT 'Asia/Almaty',
    avatar_color  text NOT NULL DEFAULT '#1AC6FF',
    disabled_at   timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_lower_uq ON users (lower(email));
CREATE INDEX users_role_idx ON users (role);

-- Session tokens are stored only as SHA-256 hashes.
CREATE TABLE sessions (
    token_hash   bytea PRIMARY KEY,
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    user_agent   text NOT NULL DEFAULT '',
    ip           text NOT NULL DEFAULT ''
);
CREATE INDEX sessions_user_idx ON sessions (user_id);
CREATE INDEX sessions_expires_idx ON sessions (expires_at);

CREATE TABLE login_attempts (
    id           bigserial PRIMARY KEY,
    email        text NOT NULL,
    ip           text NOT NULL,
    success      boolean NOT NULL,
    attempted_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX login_attempts_email_idx ON login_attempts (lower(email), attempted_at);
CREATE INDEX login_attempts_ip_idx ON login_attempts (ip, attempted_at);

-- Admin assigns teachers to students.
CREATE TABLE teacher_students (
    teacher_id  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    student_id  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    assigned_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (teacher_id, student_id)
);
CREATE INDEX teacher_students_student_idx ON teacher_students (student_id);

-- +goose Down
DROP TABLE teacher_students;
DROP TABLE login_attempts;
DROP TABLE sessions;
DROP TABLE users;
