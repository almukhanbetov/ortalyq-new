-- +goose Up
-- Marathons, stage 10: appearance.
-- * cover_attachment_id: a cover picture of the marathon (classrooms) and of a lesson in the marathon
--   (marathon_lessons). The picture is an attachment of the marathon class (uploads): only its members can open
--   it. Without a picture the generated cover (colour + letter) is shown, as before.
-- * lesson_appearance: «Оформление урока» — a lesson with its own appearance uses it instead of the marathon's.
-- * marathon_appearances: «Реакции на ответы» and «Фон в классе». lesson_id NULL is the marathon itself; a row
--   with lesson_id is the lesson's own appearance in this marathon (enabled = «Своё оформление урока»), never the
--   lesson template, so other classes using the same lesson are not affected.
--   reactions: icons of the correct / incorrect answer (codes of ready emoji) and sounds 1–5, validated by the
--   service. background: a ready variant or 'custom' (a picture in background_attachment_id).
-- purpose: pictures of the appearance cannot be sent to the chat (and so never removed with a chat message).
ALTER TABLE attachments ADD COLUMN purpose text NOT NULL DEFAULT 'chat' CHECK (purpose IN ('chat', 'appearance'));
ALTER TABLE classrooms ADD COLUMN cover_attachment_id uuid REFERENCES attachments(id) ON DELETE SET NULL;
ALTER TABLE marathon_lessons ADD COLUMN cover_attachment_id uuid REFERENCES attachments(id) ON DELETE SET NULL;
ALTER TABLE marathon_settings ADD COLUMN lesson_appearance boolean NOT NULL DEFAULT false;

CREATE TABLE marathon_appearances (
    id                       bigserial PRIMARY KEY,
    classroom_id             uuid        NOT NULL REFERENCES classrooms(id) ON DELETE CASCADE,
    lesson_id                uuid,
    enabled                  boolean     NOT NULL DEFAULT true,
    reactions_enabled        boolean     NOT NULL DEFAULT false,
    reactions                jsonb       NOT NULL,
    background_enabled       boolean     NOT NULL DEFAULT false,
    background               text        NOT NULL DEFAULT 'plain'
                             CHECK (background IN ('plain', 'sunrise', 'waves', 'petals', 'leaves', 'autumn', 'custom')),
    background_attachment_id uuid        REFERENCES attachments(id) ON DELETE SET NULL,
    updated_by               uuid        REFERENCES users(id) ON DELETE SET NULL,
    updated_at               timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (classroom_id, lesson_id) REFERENCES marathon_lessons (classroom_id, lesson_id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX marathon_appearances_uq
    ON marathon_appearances (classroom_id, COALESCE(lesson_id, '00000000-0000-0000-0000-000000000000'::uuid));

-- +goose Down
-- Pictures stay in attachments (and on disk) as ordinary class files; only the references are dropped.
DROP TABLE marathon_appearances;
ALTER TABLE marathon_settings DROP COLUMN lesson_appearance;
ALTER TABLE marathon_lessons DROP COLUMN cover_attachment_id;
ALTER TABLE classrooms DROP COLUMN cover_attachment_id;
ALTER TABLE attachments DROP COLUMN purpose;
