DROP INDEX IF EXISTS idx_point_tx_reverses_id;
ALTER TABLE point_transactions DROP COLUMN reverses_id;
