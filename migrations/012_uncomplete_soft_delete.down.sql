-- Rollback: drop the partial index first, then the column. Rows that happen
-- to be soft-deleted (uncompleted_at IS NOT NULL) are LEFT IN PLACE — old
-- code without this column will treat them as completed again, which is the
-- correct behavior for a rollback. Deleting approved completions + photos +
-- AI feedback just because they happen to be toggled off at rollback time
-- would be destructive and surprising.
DROP INDEX IF EXISTS idx_completions_uncompleted_at_null;
ALTER TABLE chore_completions DROP COLUMN uncompleted_at;
