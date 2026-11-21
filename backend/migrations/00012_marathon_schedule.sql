-- +goose Up
-- Marathons, stage 5 (continued): schedules of "По интервалам" and "По дням недели".
-- Rule ORTALYQ (accepted Н1 = A): a student's schedule counts from the anchor = max(start_at, activated_at);
-- lesson 0 opens at the anchor itself, the next lessons follow the schedule (see marathons/access.go).
-- * interval_days: "Раз в N дней", 1..7.
-- * weekdays: bit mask Monday = 1 … Sunday = 64 (1..127).
-- * open_time: "в Время", a whole hour in the marathon time zone (the original offers 00:00 … 23:00).
-- * marathon_mode_changes.dates_reset: how many lesson opening dates were reset (with explicit confirmation).
ALTER TABLE marathon_settings ADD COLUMN interval_days smallint CHECK (interval_days BETWEEN 1 AND 7);
ALTER TABLE marathon_settings ADD COLUMN weekdays smallint CHECK (weekdays BETWEEN 1 AND 127);
ALTER TABLE marathon_settings ADD COLUMN open_time time CHECK (open_time IS NULL OR (extract(minute FROM open_time) = 0 AND extract(second FROM open_time) = 0));
ALTER TABLE marathon_settings ADD CONSTRAINT marathon_settings_mode_interval_check
    CHECK (mode <> 'interval' OR (interval_days IS NOT NULL AND open_time IS NOT NULL));
ALTER TABLE marathon_settings ADD CONSTRAINT marathon_settings_mode_weekdays_check
    CHECK (mode <> 'weekdays' OR (weekdays IS NOT NULL AND open_time IS NOT NULL));
ALTER TABLE marathon_mode_changes ADD COLUMN dates_reset integer NOT NULL DEFAULT 0;

-- +goose Down
UPDATE marathon_settings SET mode = 'open' WHERE mode IN ('interval', 'weekdays');
ALTER TABLE marathon_mode_changes DROP COLUMN dates_reset;
ALTER TABLE marathon_settings DROP CONSTRAINT marathon_settings_mode_weekdays_check;
ALTER TABLE marathon_settings DROP CONSTRAINT marathon_settings_mode_interval_check;
ALTER TABLE marathon_settings DROP COLUMN open_time;
ALTER TABLE marathon_settings DROP COLUMN weekdays;
ALTER TABLE marathon_settings DROP COLUMN interval_days;
