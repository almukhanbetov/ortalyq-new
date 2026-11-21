-- +goose Up
-- Stage 1 of marathons: a marathon is a classroom of type 'marathon' with its own settings row.
-- Passing modes, dates, lessons and enrollments are added by later migrations.
ALTER TABLE classrooms DROP CONSTRAINT classrooms_lesson_type_check;
ALTER TABLE classrooms ADD CONSTRAINT classrooms_lesson_type_check
    CHECK (lesson_type IN ('individual', 'group', 'marathon'));

-- "Delete marathon" archives it: the row and all history stay, the class disappears for everyone.
ALTER TABLE classrooms ADD COLUMN archived_at timestamptz;
ALTER TABLE classrooms ADD COLUMN archived_by uuid REFERENCES users(id) ON DELETE SET NULL;

CREATE TABLE marathon_settings (
    classroom_id    uuid PRIMARY KEY REFERENCES classrooms(id) ON DELETE CASCADE,
    direction       text NOT NULL DEFAULT 'languages' CHECK (direction IN ('languages', 'other')),
    target_language text CHECK (target_language IS NULL OR target_language ~ '^[a-z]{2}$'),
    -- Owner's time zone at creation; used later to open lessons by calendar day and time.
    timezone        text NOT NULL,
    mode            text NOT NULL DEFAULT 'open'
                    CHECK (mode IN ('open', 'interval', 'progressive', 'closed', 'weekdays')),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CHECK ((direction = 'languages') = (target_language IS NOT NULL))
);

-- +goose Down
DROP TABLE marathon_settings;
DELETE FROM classrooms WHERE lesson_type = 'marathon';
ALTER TABLE classrooms DROP COLUMN archived_by;
ALTER TABLE classrooms DROP COLUMN archived_at;
ALTER TABLE classrooms DROP CONSTRAINT classrooms_lesson_type_check;
ALTER TABLE classrooms ADD CONSTRAINT classrooms_lesson_type_check
    CHECK (lesson_type IN ('individual', 'group'));
