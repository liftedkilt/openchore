-- Let a trigger post its chore more than once a day (e.g. one "put away the
-- laundry" per dryer load). Off by default: a trigger still refuses to post
-- a chore that is already on today's schedule unless this is set.
ALTER TABLE chore_triggers ADD COLUMN allow_repeat INTEGER NOT NULL DEFAULT 0;
