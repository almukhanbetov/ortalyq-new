-- +goose Up
-- «Создать материал» (Основные / Теги): a course gets a cover picture, a language, the «Разделы» switch and tags.
--
-- * course_covers: a picture uploaded from the form (PNG, JPEG or WebP, checked by the uploads module and stored
--   outside any public directory). Only its uploader attaches it to a course and reads it.
-- * language: a code of the project language list (internal/languages, the same list as marathons) or '' when
--   not chosen. The list is checked by the service, the table keeps the code format only.
-- * sections_enabled («Разделы»): on — the teacher divides each lesson into named sections (the existing structure);
--   off — a new lesson gets one section and the lesson editor shows its exercises as one list. Existing courses
--   already use sections, so they keep it on; the form starts with it off for a new material.
-- * tags: the teacher's own labels of the material.
CREATE TABLE course_covers (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stored_name text        NOT NULL UNIQUE,
    mime_type   text        NOT NULL CHECK (mime_type IN ('image/png', 'image/jpeg', 'image/webp')),
    size_bytes  bigint      NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 5242880),
    width       integer     NOT NULL,
    height      integer     NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX course_covers_owner_idx ON course_covers (owner_id);

ALTER TABLE courses
    ADD COLUMN cover_image_id   uuid REFERENCES course_covers(id) ON DELETE SET NULL,
    ADD COLUMN language         text    NOT NULL DEFAULT '' CHECK (language = '' OR language ~ '^[a-z]{2}$'),
    ADD COLUMN sections_enabled boolean NOT NULL DEFAULT true,
    ADD COLUMN tags             text[]  NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE courses DROP COLUMN tags, DROP COLUMN sections_enabled, DROP COLUMN language, DROP COLUMN cover_image_id;
DROP TABLE course_covers;
