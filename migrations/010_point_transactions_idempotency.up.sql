-- Add an idempotency key to point_transactions so callers can safely
-- retry operations (e.g. missed-chore penalties) without the fragility
-- of matching free-text notes via LIKE. A partial UNIQUE index enforces
-- at-most-one transaction per key while still allowing historical rows
-- (and any operations that don't need idempotency) to leave the column
-- NULL.

ALTER TABLE point_transactions ADD COLUMN idempotency_key TEXT;

CREATE UNIQUE INDEX idx_point_tx_idempotency_key
    ON point_transactions(idempotency_key)
    WHERE idempotency_key IS NOT NULL;
