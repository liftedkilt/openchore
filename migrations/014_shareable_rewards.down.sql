-- Drop any active commitment rows tied to a shared pool — the old per-user
-- partial unique index can't see the shared_pool_id discriminator and a
-- personal-active row already exists for these users in the typical case.
-- Cancelled / redeemed shared rows survive (their status keeps them out of
-- the index predicate).
UPDATE reward_commitments
   SET status = 'cancelled', cancelled_at = CURRENT_TIMESTAMP
 WHERE status = 'active' AND shared_pool_id IS NOT NULL;

DROP INDEX IF EXISTS idx_reward_commitments_one_active_share_per_pool;
DROP INDEX IF EXISTS idx_reward_commitments_one_active_personal;
DROP INDEX IF EXISTS idx_reward_commitments_pool;

-- Restore the original "one active commitment per user" rule.
CREATE UNIQUE INDEX idx_reward_commitments_one_active
    ON reward_commitments(user_id)
    WHERE status = 'active';

ALTER TABLE reward_commitments DROP COLUMN shared_pool_id;

DROP INDEX IF EXISTS idx_shared_pools_status;
DROP INDEX IF EXISTS idx_shared_pools_one_active_per_reward;
DROP TABLE IF EXISTS shared_commitment_pools;

ALTER TABLE rewards DROP COLUMN shareable;
