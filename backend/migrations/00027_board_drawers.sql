-- +goose Up
-- Drawing together on the lesson board. The teacher of the class lets a chosen student draw; the permission belongs
-- to the board open in the class right now: it goes away when the board is closed (cascade) and the service clears
-- it when another board is opened. board_id is kept so a permission can never apply to a different board.
CREATE TABLE classroom_board_drawers (
    classroom_id uuid        NOT NULL REFERENCES classroom_boards(classroom_id) ON DELETE CASCADE,
    user_id      uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    board_id     uuid        NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    granted_by   uuid        REFERENCES users(id) ON DELETE SET NULL,
    granted_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (classroom_id, user_id)
);

-- A student's stroke is added to the board as its own action (not by saving the whole board), so drawing at the same
-- time never overwrites the teacher's changes. client_id makes a repeated send of the same stroke a no-op.
CREATE TABLE board_stroke_receipts (
    board_id   uuid        NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    client_id  uuid        NOT NULL,
    user_id    uuid        REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (board_id, client_id)
);

-- +goose Down
DROP TABLE board_stroke_receipts;
DROP TABLE classroom_board_drawers;
