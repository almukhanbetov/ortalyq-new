-- +goose Up
-- Marathons, stage 9: invitation by e-mail and the mail outbox.
-- * email_outbox: every letter is written here inside the business transaction and sent after the commit by the
--   mail dispatcher (retries with a growing delay, "failed" after the attempt limit). The body is replaced after
--   sending or the final failure, so one-time links do not stay in the table.
-- * marathon_invitations: an invitation of a student by e-mail. Only the SHA-256 of the token is stored; the
--   plain token exists only in the letter. One open invitation per marathon and address (a repeated invitation
--   does not create a second row). Accepting enrolls the signed-in student whose e-mail matches.
CREATE TABLE email_outbox (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    kind            text        NOT NULL,
    ref_id          uuid,
    to_email        text        NOT NULL,
    locale          text        NOT NULL,
    subject         text        NOT NULL,
    body            text        NOT NULL,
    status          text        NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'sent', 'failed')),
    attempts        integer     NOT NULL DEFAULT 0,
    last_error      text,
    provider_id     text,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    sent_at         timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX email_outbox_due_idx ON email_outbox (next_attempt_at) WHERE status = 'queued';

CREATE TABLE marathon_invitations (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    classroom_id uuid        NOT NULL REFERENCES classrooms(id) ON DELETE CASCADE,
    email        text        NOT NULL CHECK (email = lower(email)),
    student_id   uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    locale       text        NOT NULL CHECK (locale IN ('ru', 'kk', 'en')),
    token_hash   bytea       NOT NULL UNIQUE,
    invited_by   uuid        REFERENCES users(id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    sent_at      timestamptz NOT NULL DEFAULT now(),
    sends        integer     NOT NULL DEFAULT 1,
    expires_at   timestamptz NOT NULL,
    outbox_id    uuid        REFERENCES email_outbox(id) ON DELETE SET NULL,
    accepted_at  timestamptz,
    accepted_by  uuid        REFERENCES users(id) ON DELETE SET NULL,
    revoked_at   timestamptz,
    revoked_by   uuid        REFERENCES users(id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX marathon_invitations_open_uq ON marathon_invitations (classroom_id, email)
    WHERE accepted_at IS NULL AND revoked_at IS NULL;

-- +goose Down
DROP TABLE marathon_invitations;
DROP TABLE email_outbox;
