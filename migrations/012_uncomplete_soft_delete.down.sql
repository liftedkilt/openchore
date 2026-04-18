-- Drop the soft-delete column. Any currently-uncompleted (soft-deleted) rows
-- stop being hidden by reader queries, but the app code on the "down" side
-- doesn't know about this column so the rows would look "completed" again.
-- Clear those rows first so the state is sensible after rollback.
DELETE FROM chore_completions WHERE uncompleted_at IS NOT NULL;
ALTER TABLE chore_completions DROP COLUMN uncompleted_at;
