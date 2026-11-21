-- +goose Up
-- Marathons, stage 3: manual activation/deactivation of students and removal from a marathon.
-- * deactivated_at/by: set while the student is deactivated by the teacher; the server then refuses every
--   lesson action of this student in this marathon (list, direct lesson URL, attempts API).
-- * reactivated_at/by: the last manual activation.
-- * removed_at/by: the student was removed from the marathon. Their classroom membership is deleted,
--   answers, attempts and progress stay; adding the student again restores them.
-- * marathon_enrollment_events: who did what and when (the columns keep only the current state).

ALTER TABLE marathon_enrollments ADD COLUMN deactivated_at timestamptz;
ALTER TABLE marathon_enrollments ADD COLUMN deactivated_by uuid REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE marathon_enrollments ADD COLUMN reactivated_at timestamptz;
ALTER TABLE marathon_enrollments ADD COLUMN reactivated_by uuid REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE marathon_enrollments ADD COLUMN removed_at timestamptz;
ALTER TABLE marathon_enrollments ADD COLUMN removed_by uuid REFERENCES users(id) ON DELETE SET NULL;

CREATE TABLE marathon_enrollment_events (
    id           bigserial   PRIMARY KEY,
    classroom_id uuid        NOT NULL REFERENCES classrooms(id) ON DELETE CASCADE,
    student_id   uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event        text        NOT NULL CHECK (event IN ('added', 'activated', 'deactivated', 'removed')),
    actor_id     uuid        REFERENCES users(id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX marathon_enrollment_events_idx ON marathon_enrollment_events (classroom_id, student_id, created_at);

-- +goose Down
-- Before this migration a removed enrollment did not exist (its membership is already gone).
DELETE FROM marathon_enrollments WHERE removed_at IS NOT NULL;
DROP TABLE marathon_enrollment_events;
ALTER TABLE marathon_enrollments DROP COLUMN removed_by;
ALTER TABLE marathon_enrollments DROP COLUMN removed_at;
ALTER TABLE marathon_enrollments DROP COLUMN reactivated_by;
ALTER TABLE marathon_enrollments DROP COLUMN reactivated_at;
ALTER TABLE marathon_enrollments DROP COLUMN deactivated_by;
ALTER TABLE marathon_enrollments DROP COLUMN deactivated_at;
