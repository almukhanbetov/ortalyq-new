-- +goose Up
-- «Материалы» → «Интерактивные доски»: a teacher's personal whiteboards. The drawing is one JSON document
-- (strokes and texts, validated by the boards module); version grows with every save so a stale tab cannot silently
-- overwrite a newer save. Boards are private to their owner at this stage.
CREATE TABLE boards (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id   uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title      text        NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    content    jsonb       NOT NULL DEFAULT '{"items":[]}'::jsonb,
    version    integer     NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX boards_owner_idx ON boards (owner_id, updated_at DESC);

-- +goose Down
DROP TABLE boards;
