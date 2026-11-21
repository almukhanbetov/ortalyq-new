-- +goose Up
-- Marathons, stage 7: statistics and rating.
-- * stats_enabled: "Статистика" — the statistics tab, the export and the lesson results are available.
-- * rating_enabled: "Рейтинг учеников" — only together with the statistics (as in the original, the switch is
--   locked while the statistics is off).
-- * rating_limit: "Отображать" — first 3 / 5 / 10 places for students; NULL = all students.
-- * rating_show_place: "Место ученика" — a student outside the shown places sees their own place.
-- * marathon_lesson_comments: «Комментарий к статистике» of a lesson. Decision Н7: it belongs to the lesson in
--   this marathon, not to the lesson template, so it never appears in other classes using the same lesson.
-- Points are never stored: they are recomputed from answers (attempts.Points), like everywhere else.
ALTER TABLE marathon_settings ADD COLUMN stats_enabled boolean NOT NULL DEFAULT false;
ALTER TABLE marathon_settings ADD COLUMN rating_enabled boolean NOT NULL DEFAULT false;
ALTER TABLE marathon_settings ADD COLUMN rating_limit smallint CHECK (rating_limit IN (3, 5, 10));
ALTER TABLE marathon_settings ADD COLUMN rating_show_place boolean NOT NULL DEFAULT false;
ALTER TABLE marathon_settings ADD CONSTRAINT marathon_settings_rating_check CHECK (NOT rating_enabled OR stats_enabled);

CREATE TABLE marathon_lesson_comments (
    classroom_id uuid        NOT NULL,
    lesson_id    uuid        NOT NULL,
    body         text        NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
    updated_by   uuid        REFERENCES users(id) ON DELETE SET NULL,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (classroom_id, lesson_id),
    FOREIGN KEY (classroom_id, lesson_id) REFERENCES marathon_lessons (classroom_id, lesson_id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE marathon_lesson_comments;
ALTER TABLE marathon_settings DROP CONSTRAINT marathon_settings_rating_check;
ALTER TABLE marathon_settings DROP COLUMN rating_show_place;
ALTER TABLE marathon_settings DROP COLUMN rating_limit;
ALTER TABLE marathon_settings DROP COLUMN rating_enabled;
ALTER TABLE marathon_settings DROP COLUMN stats_enabled;
