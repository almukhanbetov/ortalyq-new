-- +goose Up
-- «Материалы» → «Личные материалы» → «Загрузить PDF»: a PDF document is a personal material of the teacher who
-- uploaded it, next to their courses. Only the owner lists, opens and reads it (also by a direct link): it is not
-- shared with classes or other teachers at this stage.
--
-- The file is checked (a real PDF, at most 20 MB) and written by the uploads module outside any public directory;
-- the API serves it only through the document after the owner check, never by its stored name.
CREATE TABLE material_documents (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id      uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title         text        NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    original_name text        NOT NULL,
    stored_name   text        NOT NULL UNIQUE,
    mime_type     text        NOT NULL CHECK (mime_type = 'application/pdf'),
    size_bytes    bigint      NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 20971520),
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX material_documents_owner_idx ON material_documents (owner_id, created_at DESC);

-- +goose Down
DROP TABLE material_documents;
