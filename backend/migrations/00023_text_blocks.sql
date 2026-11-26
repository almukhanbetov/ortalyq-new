-- +goose Up
-- «Текст» (kind 'text'): a read-only block of a lesson section for reading and speaking practice. Its text is the
-- prompt of its single question (up to 2000 characters, the existing column limit); it has no options, no attempts
-- and no points. Text blocks live only in the lesson scope, never in homework.
ALTER TABLE exercises DROP CONSTRAINT exercises_kind_check;
ALTER TABLE exercises ADD CONSTRAINT exercises_kind_check CHECK (kind IN ('test', 'gaps', 'text'));
ALTER TABLE exercises ADD CONSTRAINT exercises_text_lesson_only CHECK (kind <> 'text' OR scope = 'lesson');

-- +goose Down
ALTER TABLE exercises DROP CONSTRAINT exercises_text_lesson_only;
ALTER TABLE exercises DROP CONSTRAINT exercises_kind_check;
-- NOT VALID keeps text blocks created meanwhile instead of failing the rollback; new rows are checked again.
ALTER TABLE exercises ADD CONSTRAINT exercises_kind_check CHECK (kind IN ('test', 'gaps')) NOT VALID;
