-- +goose Up
-- «PDF-документ» (kind 'pdf'): a read-only block of a lesson section that shows a PDF file. Like «Текст» it has no
-- attempts and no points and lives only in the lesson scope, never in homework.
--
-- The file is checked and written by the uploads module (a real PDF, at most 20 MB) outside any public directory;
-- lesson_files records it with its uploader (not with a lesson: a copied lesson shares the row, and deleting the
-- source lesson must not take the copy's file). A version refers to its file by file_id, so a new version with another file keeps the
-- old one for the old version, and a copied lesson (marathons) refers to the same file row. The file is never served
-- by its own id: the API opens it through the exercise, with the exercise's access rules (the lesson owner; a member
-- of a class in which the lesson is open, the block is not hidden and the marathon admits the student).
ALTER TABLE exercises DROP CONSTRAINT exercises_kind_check;
ALTER TABLE exercises ADD CONSTRAINT exercises_kind_check CHECK (kind IN ('test', 'gaps', 'text', 'pdf'));
ALTER TABLE exercises ADD CONSTRAINT exercises_pdf_lesson_only CHECK (kind <> 'pdf' OR scope = 'lesson');

CREATE TABLE lesson_files (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id      uuid        REFERENCES users(id) ON DELETE SET NULL, -- who uploaded it: only they attach it to a block
    original_name text        NOT NULL,
    stored_name   text        NOT NULL UNIQUE,
    mime_type     text        NOT NULL CHECK (mime_type = 'application/pdf'),
    size_bytes    bigint      NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 20971520),
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX lesson_files_owner_idx ON lesson_files (owner_id);

ALTER TABLE exercise_versions ADD COLUMN file_id uuid REFERENCES lesson_files(id) ON DELETE RESTRICT;

-- +goose Down
ALTER TABLE exercise_versions DROP COLUMN file_id;
DROP TABLE lesson_files;
ALTER TABLE exercises DROP CONSTRAINT exercises_pdf_lesson_only;
ALTER TABLE exercises DROP CONSTRAINT exercises_kind_check;
-- NOT VALID keeps PDF blocks created meanwhile instead of failing the rollback; new rows are checked again.
ALTER TABLE exercises ADD CONSTRAINT exercises_kind_check CHECK (kind IN ('test', 'gaps', 'text')) NOT VALID;
