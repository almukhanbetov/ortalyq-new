-- +goose Up
-- «Слова и пропуски» (kind 'gaps'), first step: one gap per exercise.
-- The phrase with the gap marker (___) is the question prompt; the accepted word is its only option (is_correct),
-- so versions, copying and statistics keep working on the existing tables. The student's typed word is stored
-- next to the chosen options of an answer; a test answer leaves it empty.
ALTER TABLE exercises DROP CONSTRAINT exercises_kind_check;
ALTER TABLE exercises ADD CONSTRAINT exercises_kind_check CHECK (kind IN ('test', 'gaps'));

ALTER TABLE attempt_answers ADD COLUMN answer_text text NOT NULL DEFAULT '' CHECK (length(answer_text) <= 200);
ALTER TABLE preview_answers ADD COLUMN answer_text text NOT NULL DEFAULT '' CHECK (length(answer_text) <= 200);

-- +goose Down
ALTER TABLE preview_answers DROP COLUMN answer_text;
ALTER TABLE attempt_answers DROP COLUMN answer_text;
ALTER TABLE exercises DROP CONSTRAINT exercises_kind_check;
-- NOT VALID keeps gap exercises created meanwhile instead of failing the rollback; new rows are checked again.
ALTER TABLE exercises ADD CONSTRAINT exercises_kind_check CHECK (kind = 'test') NOT VALID;
