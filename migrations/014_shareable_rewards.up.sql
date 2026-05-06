-- Shared / family reward goals: multiple kids can pool points toward one
-- shareable reward (think: family Minecraft purchase) and see what each
-- person has chipped in. Each kid keeps a personal commitment slot too —
-- shared participation is additive.
--
-- Modelling:
--   * rewards.shareable = 1 marks a reward as poolable. The first kid to
--     commit creates a shared_commitment_pools row that snapshots the cost
--     so admins changing the price can't punish savers mid-pool.
--   * reward_commitments grows a nullable shared_pool_id. NULL = personal
--     commitment (existing behaviour). Set = a kid's share of a pool.
--   * The "one active commitment per user" partial unique index is relaxed:
--     it now only covers personal commitments (shared_pool_id IS NULL), so a
--     kid can have one personal goal AND join multiple shared pools.

ALTER TABLE rewards ADD COLUMN shareable INTEGER NOT NULL DEFAULT 0
    CHECK (shareable IN (0, 1));

CREATE TABLE shared_commitment_pools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    reward_id INTEGER NOT NULL REFERENCES rewards(id),
    target_cost INTEGER NOT NULL CHECK (target_cost > 0),
    status TEXT NOT NULL CHECK (status IN ('active', 'redeemed', 'cancelled'))
        DEFAULT 'active',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    redeemed_at DATETIME,
    redeemed_by_user_id INTEGER REFERENCES users(id),
    cancelled_at DATETIME
);

-- At most one active pool per reward at a time. Once redeemed/cancelled, a
-- new pool can be opened for the same reward (next family Minecraft round).
CREATE UNIQUE INDEX idx_shared_pools_one_active_per_reward
    ON shared_commitment_pools(reward_id)
    WHERE status = 'active';

CREATE INDEX idx_shared_pools_status
    ON shared_commitment_pools(status);

ALTER TABLE reward_commitments ADD COLUMN shared_pool_id INTEGER
    REFERENCES shared_commitment_pools(id);

CREATE INDEX idx_reward_commitments_pool
    ON reward_commitments(shared_pool_id)
    WHERE shared_pool_id IS NOT NULL;

-- Replace the "one active commitment per user" rule with one that only
-- applies to personal commitments. Shared shares are unrestricted.
DROP INDEX idx_reward_commitments_one_active;

CREATE UNIQUE INDEX idx_reward_commitments_one_active_personal
    ON reward_commitments(user_id)
    WHERE status = 'active' AND shared_pool_id IS NULL;

-- A user can have at most one active share in any given pool (rejoining
-- after breaking restarts a fresh row but the prior is cancelled).
CREATE UNIQUE INDEX idx_reward_commitments_one_active_share_per_pool
    ON reward_commitments(user_id, shared_pool_id)
    WHERE status = 'active' AND shared_pool_id IS NOT NULL;
