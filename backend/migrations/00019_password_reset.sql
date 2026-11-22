-- +goose Up
-- Password reset by e-mail: request → one-time link (only the SHA-256 of the token is stored) → new password →
-- sign-in screen.
-- * One row per request, also for an address without an account (user_id and token_hash are NULL and no letter is
--   sent): the rows drive the limits per address and per IP, so the limits answer the same whether or not the
--   account exists.
-- * A newer request supersedes the older open link of the account; a successful reset closes the link (used_at)
--   and the other open links of the account.
CREATE TABLE password_resets (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text        NOT NULL CHECK (email = lower(email)),
    locale        text        NOT NULL CHECK (locale IN ('ru', 'kk', 'en')),
    user_id       uuid        REFERENCES users(id) ON DELETE CASCADE,
    token_hash    bytea       UNIQUE,
    ip            text        NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL,
    superseded_at timestamptz,
    used_at       timestamptz,
    outbox_id     uuid        REFERENCES email_outbox(id) ON DELETE SET NULL,
    CHECK ((user_id IS NULL) = (token_hash IS NULL))
);
CREATE INDEX password_resets_email_idx ON password_resets (email, created_at);
CREATE INDEX password_resets_ip_idx ON password_resets (ip, created_at);
CREATE INDEX password_resets_user_idx ON password_resets (user_id) WHERE used_at IS NULL AND superseded_at IS NULL;

-- +goose Down
DROP TABLE password_resets;
