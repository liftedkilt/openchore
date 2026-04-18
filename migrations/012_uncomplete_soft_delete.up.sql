-- Soft-delete support for chore_completions so that an accidental uncheck +
-- recheck preserves approved photo + AI feedback + approval metadata for the
-- same schedule + date. A non-null uncompleted_at means the completion is
-- currently treated as "not completed" by the UI, but the data survives so
-- the user can toggle back on without redoing the photo / AI review.
ALTER TABLE chore_completions ADD COLUMN uncompleted_at DATETIME;

-- Hot-path readers (GetScheduledChoresForUser, FCFS checks, daily summaries)
-- all filter on uncompleted_at IS NULL. A partial index avoids scanning the
-- long tail of soft-deleted rows that accumulate over time. SQLite has
-- supported partial indexes since 3.8, and modernc.org/sqlite ships a
-- recent-enough version.
CREATE INDEX idx_completions_uncompleted_at_null
  ON chore_completions(chore_schedule_id, completion_date)
  WHERE uncompleted_at IS NULL;
