DROP INDEX IF EXISTS idx_point_tx_idempotency_key;

ALTER TABLE point_transactions DROP COLUMN idempotency_key;
