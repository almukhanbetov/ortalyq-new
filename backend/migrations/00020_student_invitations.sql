-- +goose Up
-- Invitations of a private teacher to a student by e-mail.
-- * One row per letter. Only the SHA-256 of the token is stored. A newer letter from the same teacher to the same
--   address supersedes the older open link; a link works once (accepted_at) and until expires_at.
-- * An accepted invitation links the student to the teacher in teacher_students (assigned_by = the teacher,
--   invitation_id = the row here). School teachers keep getting students from the administrator only.
CREATE TABLE student_invitations (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    teacher_id    uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email         text        NOT NULL CHECK (email = lower(email)),
    locale        text        NOT NULL DEFAULT 'ru' CHECK (locale IN ('ru', 'kk', 'en')),
    token_hash    bytea       NOT NULL UNIQUE,
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL,
    superseded_at timestamptz,
    accepted_at   timestamptz,
    accepted_by   uuid        REFERENCES users(id) ON DELETE SET NULL,
    outbox_id     uuid
);
CREATE INDEX student_invitations_teacher_idx ON student_invitations (teacher_id, created_at DESC);
CREATE INDEX student_invitations_email_idx ON student_invitations (teacher_id, email) WHERE accepted_at IS NULL AND superseded_at IS NULL;

ALTER TABLE teacher_students ADD COLUMN invitation_id uuid REFERENCES student_invitations(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE teacher_students DROP COLUMN invitation_id;
DROP TABLE student_invitations;
