-- +goose Up
CREATE TABLE attachments (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    classroom_id  uuid NOT NULL REFERENCES classrooms(id) ON DELETE CASCADE,
    uploader_id   uuid REFERENCES users(id) ON DELETE SET NULL,
    original_name text NOT NULL,
    stored_name   text NOT NULL UNIQUE,
    mime_type     text NOT NULL,
    size_bytes    bigint NOT NULL CHECK (size_bytes > 0),
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX attachments_classroom_idx ON attachments (classroom_id);

CREATE TABLE chat_messages (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    classroom_id  uuid NOT NULL REFERENCES classrooms(id) ON DELETE CASCADE,
    author_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id     uuid NOT NULL,
    kind          text NOT NULL CHECK (kind IN ('text', 'gif', 'file')),
    body          text NOT NULL DEFAULT '',
    gif           jsonb,
    attachment_id uuid REFERENCES attachments(id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    edited_at     timestamptz,
    deleted_at    timestamptz,
    UNIQUE (author_id, client_id)
);
CREATE INDEX chat_messages_classroom_idx ON chat_messages (classroom_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE chat_messages;
DROP TABLE attachments;
