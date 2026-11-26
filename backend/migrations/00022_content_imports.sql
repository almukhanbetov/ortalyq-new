-- +goose Up
-- Ready-made author content loaded by `api seed-course` (internal/content). One row per teacher and content key
-- («english-a1-first-conversations», «english-a1-first-conversations/hello», …) remembers that the course or the
-- lesson was already given to this teacher. A repeated load skips such keys: it creates no duplicates and never
-- touches a lesson the teacher has edited. entity_id has no foreign key on purpose: when the teacher deletes the
-- lesson, the row stays and the lesson does not come back on the next load.
CREATE TABLE content_imports (
    owner_id    uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key         text        NOT NULL CHECK (length(key) BETWEEN 1 AND 200),
    entity_id   uuid        NOT NULL,
    imported_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (owner_id, key)
);

-- +goose Down
DROP TABLE content_imports;
