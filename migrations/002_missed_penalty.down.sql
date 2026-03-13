-- SQLite does not support dropping columns easily, but we can't do much about it in a simple migration.
-- Usually one would recreate the table, but for a dev project this is often skipped for "down" migrations.
-- However, for correctness:
PRAGMA foreign_keys=OFF;
CREATE TABLE chores_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL CHECK (category IN ('required', 'core', 'bonus')) DEFAULT 'core',
    icon TEXT NOT NULL DEFAULT '',
    points_value INTEGER NOT NULL DEFAULT 0,
    estimated_minutes INTEGER,
    source TEXT NOT NULL DEFAULT 'manual',
    external_id TEXT NOT NULL DEFAULT '',
    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO chores_new (id, title, description, category, icon, points_value, estimated_minutes, source, external_id, created_by, created_at)
SELECT id, title, description, category, icon, points_value, estimated_minutes, source, external_id, created_by, created_at FROM chores;
DROP TABLE chores;
ALTER TABLE chores_new RENAME TO chores;
PRAGMA foreign_keys=ON;
