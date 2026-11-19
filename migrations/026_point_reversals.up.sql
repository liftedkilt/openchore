-- Undo from the Activity log writes a reversal row that points at the row it
-- cancels. Both are hidden from the history; the unique index means a row can
-- only be undone once.
ALTER TABLE point_transactions ADD COLUMN reverses_id INTEGER REFERENCES point_transactions(id) ON DELETE CASCADE;
CREATE UNIQUE INDEX idx_point_tx_reverses_id ON point_transactions(reverses_id) WHERE reverses_id IS NOT NULL;
