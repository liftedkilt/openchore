-- Soft-delete support for chore_completions so that an accidental uncheck +
-- recheck preserves approved photo + AI feedback + approval metadata for the
-- same schedule + date. A non-null uncompleted_at means the completion is
-- currently treated as "not completed" by the UI, but the data survives so
-- the user can toggle back on without redoing the photo / AI review.
ALTER TABLE chore_completions ADD COLUMN uncompleted_at DATETIME;
