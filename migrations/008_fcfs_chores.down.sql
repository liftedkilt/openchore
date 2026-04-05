-- Revert: remove FCFS assignment type and fcfs_group_id from chore_schedules.
-- Delete any FCFS schedules first, then recreate with original constraint.

DELETE FROM chore_schedules WHERE assignment_type = 'fcfs';

CREATE TABLE chore_schedules_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chore_id INTEGER NOT NULL REFERENCES chores(id) ON DELETE CASCADE,
    assigned_to INTEGER NOT NULL REFERENCES users(id),
    assignment_type TEXT NOT NULL CHECK (assignment_type IN ('individual', 'family')) DEFAULT 'individual',
    day_of_week INTEGER CHECK (day_of_week BETWEEN 0 AND 6),
    specific_date DATE,
    available_at TEXT,
    due_by TEXT,
    expiry_penalty TEXT NOT NULL DEFAULT 'block',
    expiry_penalty_value INTEGER NOT NULL DEFAULT 0,
    points_multiplier REAL NOT NULL DEFAULT 1.0,
    start_date DATE,
    end_date DATE,
    recurrence_interval INTEGER,
    recurrence_start DATE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (day_of_week IS NOT NULL OR specific_date IS NOT NULL OR recurrence_interval IS NOT NULL)
);

INSERT INTO chore_schedules_old (id, chore_id, assigned_to, assignment_type, day_of_week, specific_date, available_at, due_by, expiry_penalty, expiry_penalty_value, points_multiplier, start_date, end_date, recurrence_interval, recurrence_start, created_at)
    SELECT id, chore_id, assigned_to, assignment_type, day_of_week, specific_date, available_at, due_by, expiry_penalty, expiry_penalty_value, points_multiplier, start_date, end_date, recurrence_interval, recurrence_start, created_at
    FROM chore_schedules;

DROP TABLE chore_schedules;
ALTER TABLE chore_schedules_old RENAME TO chore_schedules;

CREATE INDEX idx_schedules_assigned ON chore_schedules(assigned_to);
CREATE INDEX idx_schedules_chore ON chore_schedules(chore_id);

-- Remove assignment_type from chore_triggers (SQLite can't DROP COLUMN in older versions,
-- but modern SQLite 3.35+ supports it)
ALTER TABLE chore_triggers DROP COLUMN assignment_type;
