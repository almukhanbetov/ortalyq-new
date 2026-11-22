-- +goose Up
-- Self sign-up of a private teacher (bilimway.kz): e-mail → one-time confirmation link → password → cabinet.
-- * users.role gains 'private_teacher': a teacher who registered on their own. Authoring rights of a teacher
--   (materials, exercises, preview, marathons), but not the school part: an administrator cannot assign school
--   students to this role (teacher_students stays 'teacher' only, checked in users.Service). Existing rows keep
--   their roles; nothing is converted.
-- * users.terms_accepted_at: when the terms and the privacy policy were accepted (NULL for accounts created by an
--   administrator before this stage).
-- * email_signups: one row per letter sent from the sign-up screen, also for an address that already has an
--   account (that letter has no link: token_hash IS NULL). The rows drive the resend limits. Only the SHA-256 of a
--   token is stored. A newer letter to the same address supersedes the older open link.
ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin', 'teacher', 'private_teacher', 'student'));
ALTER TABLE users ADD COLUMN terms_accepted_at timestamptz;

CREATE TABLE email_signups (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text        NOT NULL CHECK (email = lower(email)),
    locale        text        NOT NULL CHECK (locale IN ('ru', 'kk', 'en')),
    -- NULL: the address already had an account, the letter only suggests signing in.
    token_hash    bytea       UNIQUE,
    ip            text        NOT NULL DEFAULT '',
    terms_accepted_at timestamptz NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL,
    verified_at   timestamptz,
    superseded_at timestamptz,
    used_at       timestamptz,
    user_id       uuid        REFERENCES users(id) ON DELETE SET NULL,
    outbox_id     uuid        REFERENCES email_outbox(id) ON DELETE SET NULL
);
CREATE INDEX email_signups_email_idx ON email_signups (email, created_at);
CREATE INDEX email_signups_ip_idx ON email_signups (ip, created_at);

-- +goose Down
-- Refuses to run while self-registered teachers exist: turning them into school teachers would grant rights,
-- deleting them would lose data. Move or remove them by hand first.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM users WHERE role = 'private_teacher') THEN
        RAISE EXCEPTION 'users with role private_teacher exist; change or remove them before rolling back';
    END IF;
END $$;
-- +goose StatementEnd
DROP TABLE email_signups;
ALTER TABLE users DROP COLUMN terms_accepted_at;
ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin', 'teacher', 'student'));
