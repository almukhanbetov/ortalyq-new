-- +goose Up
-- «Создать копию для занятия»: the teacher opens a copy of a board in a class instead of the board itself. The copy
-- is a separate board of the teacher (all objects, its own copies of the pictures); request_id makes a repeated
-- request return the same copy instead of making another one.
CREATE TABLE board_copies (
    owner_id        uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    request_id      uuid        NOT NULL,
    source_board_id uuid        REFERENCES boards(id) ON DELETE SET NULL,
    copy_board_id   uuid        NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    classroom_id    uuid        REFERENCES classrooms(id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (owner_id, request_id)
);

-- +goose Down
DROP TABLE board_copies;
