-- AI no longer rejects completions: photo review only annotates pending
-- completions for a parent. Old ai_rejected rows were failed attempts that
-- never earned points, so they are removed rather than converted.
DELETE FROM chore_completions WHERE status = 'ai_rejected';

-- Older versions deleted schedules and users without cascading, leaving
-- completions that point at missing rows. Copying those into the rebuilt
-- table below fails the foreign key checks, so drop them (and clear
-- approvers that no longer exist) first.
DELETE FROM chore_completions
 WHERE chore_schedule_id NOT IN (SELECT id FROM chore_schedules)
    OR completed_by NOT IN (SELECT id FROM users);
UPDATE chore_completions SET approved_by = NULL
 WHERE approved_by IS NOT NULL AND approved_by NOT IN (SELECT id FROM users);

-- SQLite does not support ALTER CHECK, so we recreate the table.
CREATE TABLE chore_completions_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chore_schedule_id INTEGER NOT NULL REFERENCES chore_schedules(id) ON DELETE CASCADE,
    completed_by INTEGER NOT NULL REFERENCES users(id),
    status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'rejected', 'excused')) DEFAULT 'approved',
    photo_url TEXT NOT NULL DEFAULT '',
    approved_by INTEGER REFERENCES users(id),
    approved_at DATETIME,
    completed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completion_date DATE NOT NULL,
    ai_feedback TEXT NOT NULL DEFAULT '',
    ai_confidence REAL NOT NULL DEFAULT 0,
    -- The photo reviewer's read (1 done / 0 not done); NULL until reviewed.
    ai_complete INTEGER,
    uncompleted_at DATETIME
);

INSERT INTO chore_completions_new (id, chore_schedule_id, completed_by, status, photo_url, approved_by, approved_at, completed_at, completion_date, ai_feedback, ai_confidence, uncompleted_at)
    SELECT id, chore_schedule_id, completed_by, status, photo_url, approved_by, approved_at, completed_at, completion_date, ai_feedback, ai_confidence, uncompleted_at
    FROM chore_completions;

DROP TABLE chore_completions;
ALTER TABLE chore_completions_new RENAME TO chore_completions;

CREATE INDEX idx_completions_schedule_date ON chore_completions(chore_schedule_id, completion_date);
CREATE INDEX idx_completions_completed_by ON chore_completions(completed_by);
CREATE INDEX idx_completions_uncompleted_at_null
  ON chore_completions(chore_schedule_id, completion_date)
  WHERE uncompleted_at IS NULL;

-- Read-aloud audio is now generated from the chore's title and description,
-- so the LLM-rewritten spoken text goes away. Existing audio was made from
-- that rewritten text; clearing the URLs makes the server regenerate it.
ALTER TABLE chores DROP COLUMN tts_description;
UPDATE chores SET tts_audio_url = '';

-- AI connection details now come from the environment (AI_BASE_URL,
-- TTS_BASE_URL, ...); only behaviour toggles stay in settings.
UPDATE app_settings SET key = 'ai_photo_review' WHERE key = 'ai_enabled';
UPDATE app_settings SET key = 'tts_voice' WHERE key = 'ai_tts_voice';
DELETE FROM app_settings WHERE key IN ('ai_endpoint', 'ai_model', 'ai_tts_endpoint', 'ai_tts_enabled');

-- Weekly AI summaries are generated once per finished week and kept.
CREATE TABLE weekly_summaries (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    week_start DATE NOT NULL,
    summary TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, week_start)
);
