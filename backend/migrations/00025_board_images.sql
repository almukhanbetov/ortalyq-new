-- +goose Up
-- Pictures placed on a personal whiteboard. The file is checked and stored by the uploads module (the same rules as
-- class pictures: PNG, JPEG or WebP up to 5 MB); this table ties it to the board, so only the board's owner can
-- upload or read it. A board's image item refers to a row here by image_id.
CREATE TABLE board_images (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    board_id      uuid        NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    owner_id      uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    original_name text        NOT NULL,
    stored_name   text        NOT NULL UNIQUE,
    mime_type     text        NOT NULL,
    size_bytes    bigint      NOT NULL CHECK (size_bytes > 0),
    width         integer     NOT NULL CHECK (width > 0),
    height        integer     NOT NULL CHECK (height > 0),
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX board_images_board_idx ON board_images (board_id);

-- +goose Down
DROP TABLE board_images;
