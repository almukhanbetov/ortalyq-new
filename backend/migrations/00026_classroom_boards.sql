-- +goose Up
-- A personal whiteboard opened in a lesson: at most one board per classroom. While the row exists, members of the
-- classroom read this board (and only this one) through the lesson; the owner keeps editing it. Closing the board
-- removes the row: the lesson access ends, the board itself stays in the teacher's «Материалы».
CREATE TABLE classroom_boards (
    classroom_id uuid        PRIMARY KEY REFERENCES classrooms(id) ON DELETE CASCADE,
    board_id     uuid        NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    opened_by    uuid        REFERENCES users(id) ON DELETE SET NULL,
    opened_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX classroom_boards_board_idx ON classroom_boards (board_id);

-- +goose Down
DROP TABLE classroom_boards;
