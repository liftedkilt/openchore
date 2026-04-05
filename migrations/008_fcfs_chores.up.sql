-- Add FCFS (first-come-first-serve) assignment type to chore_schedules.
-- SQLite does not support ALTER CHECK, so we recreate the table.

CREATE TABLE chore_schedules_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chore_id INTEGER NOT NULL REFERENCES chores(id) ON DELETE CASCADE,
    assigned_to INTEGER NOT NULL REFERENCES users(id),
    assignment_type TEXT NOT NULL CHECK (assignment_type IN ('individual', 'family', 'fcfs')) DEFAULT 'individual',
    fcfs_group_id TEXT,
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

INSERT INTO chore_schedules_new (id, chore_id, assigned_to, assignment_type, day_of_week, specific_date, available_at, due_by, expiry_penalty, expiry_penalty_value, points_multiplier, start_date, end_date, recurrence_interval, recurrence_start, created_at)
    SELECT id, chore_id, assigned_to, assignment_type, day_of_week, specific_date, available_at, due_by, expiry_penalty, expiry_penalty_value, points_multiplier, start_date, end_date, recurrence_interval, recurrence_start, created_at
    FROM chore_schedules;

DROP TABLE chore_schedules;
ALTER TABLE chore_schedules_new RENAME TO chore_schedules;

CREATE INDEX idx_schedules_assigned ON chore_schedules(assigned_to);
CREATE INDEX idx_schedules_chore ON chore_schedules(chore_id);
CREATE INDEX idx_schedules_fcfs_group ON chore_schedules(fcfs_group_id);

-- Add assignment_type to chore_triggers
ALTER TABLE chore_triggers ADD COLUMN assignment_type TEXT NOT NULL DEFAULT 'individual';
