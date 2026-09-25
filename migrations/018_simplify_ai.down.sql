DROP TABLE weekly_summaries;

UPDATE app_settings SET key = 'ai_enabled' WHERE key = 'ai_photo_review';
UPDATE app_settings SET key = 'ai_tts_voice' WHERE key = 'tts_voice';
DELETE FROM app_settings WHERE key IN ('ai_auto_approve', 'ai_weekly_summary');

ALTER TABLE chores ADD COLUMN tts_description TEXT NOT NULL DEFAULT '';

CREATE TABLE chore_completions_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chore_schedule_id INTEGER NOT NULL REFERENCES chore_schedules(id) ON DELETE CASCADE,
    completed_by INTEGER NOT NULL REFERENCES users(id),
    status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'rejected', 'ai_rejected', 'excused')) DEFAULT 'approved',
    photo_url TEXT NOT NULL DEFAULT '',
    approved_by INTEGER REFERENCES users(id),
    approved_at DATETIME,
    completed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completion_date DATE NOT NULL,
    ai_feedback TEXT NOT NULL DEFAULT '',
    ai_confidence REAL NOT NULL DEFAULT 0,
    uncompleted_at DATETIME
);

INSERT INTO chore_completions_old (id, chore_schedule_id, completed_by, status, photo_url, approved_by, approved_at, completed_at, completion_date, ai_feedback, ai_confidence, uncompleted_at)
    SELECT id, chore_schedule_id, completed_by, status, photo_url, approved_by, approved_at, completed_at, completion_date, ai_feedback, ai_confidence, uncompleted_at
    FROM chore_completions;

DROP TABLE chore_completions;
ALTER TABLE chore_completions_old RENAME TO chore_completions;

CREATE INDEX idx_completions_schedule_date ON chore_completions(chore_schedule_id, completion_date);
CREATE INDEX idx_completions_completed_by ON chore_completions(completed_by);
CREATE INDEX idx_completions_uncompleted_at_null
  ON chore_completions(chore_schedule_id, completion_date)
  WHERE uncompleted_at IS NULL;
