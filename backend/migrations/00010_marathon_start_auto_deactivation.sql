-- +goose Up
-- Marathons, stage 4: delayed start and individual auto-deactivation.
-- * start_at: before this moment students cannot open lessons (403 marathon_not_started).
-- * auto_deactivate_days: a student loses access N days after their activated_at (the day count is made in the
--   marathon time zone). The student's deadline is min(end_at, activated_at + N days). Computed, never stored.
ALTER TABLE marathon_settings ADD COLUMN start_at timestamptz;
ALTER TABLE marathon_settings ADD COLUMN auto_deactivate_days integer CHECK (auto_deactivate_days BETWEEN 1 AND 3650);
ALTER TABLE marathon_settings ADD CONSTRAINT marathon_settings_dates_check CHECK (end_at IS NULL OR start_at IS NULL OR end_at > start_at);

-- +goose Down
ALTER TABLE marathon_settings DROP CONSTRAINT marathon_settings_dates_check;
ALTER TABLE marathon_settings DROP COLUMN auto_deactivate_days;
ALTER TABLE marathon_settings DROP COLUMN start_at;
