-- AI photo review fields on completions
ALTER TABLE chore_completions ADD COLUMN ai_feedback TEXT NOT NULL DEFAULT '';
ALTER TABLE chore_completions ADD COLUMN ai_confidence REAL NOT NULL DEFAULT 0;

-- Pre-generated TTS description for chores
ALTER TABLE chores ADD COLUMN tts_description TEXT NOT NULL DEFAULT '';
