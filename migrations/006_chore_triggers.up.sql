CREATE TABLE chore_triggers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE,
    chore_id INTEGER NOT NULL REFERENCES chores(id) ON DELETE CASCADE,
    default_assigned_to INTEGER REFERENCES users(id) ON DELETE SET NULL,
    default_due_by TEXT,
    default_available_at TEXT,
    enabled INTEGER NOT NULL DEFAULT 1,
    cooldown_minutes INTEGER NOT NULL DEFAULT 0,
    last_triggered_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX idx_chore_triggers_uuid ON chore_triggers(uuid);
