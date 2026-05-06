-- Reward commitments: lets a kid earmark points toward a chosen reward and
-- watch progress, instead of being tempted to spend their balance on cheaper
-- rewards. Implemented as a "soft" commitment: committed points are debited
-- from the kid's spendable balance via the existing point_transactions ledger
-- (reason='commit_to_goal'), and can be returned to spendable by an admin
-- breaking the goal (reason='goal_break').
--
-- Spendable balance = SUM(point_transactions.amount) for the user, which
-- naturally excludes points sitting in an active commitment. The "amount
-- saved toward goal" is derived from the same ledger by summing rows that
-- reference a specific commitment.
--
-- One active commitment per user keeps the UX simple for younger kids; the
-- partial UNIQUE index enforces it. Redeemed/cancelled rows stay around for
-- history.

CREATE TABLE reward_commitments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reward_id INTEGER NOT NULL REFERENCES rewards(id),
    target_cost INTEGER NOT NULL CHECK (target_cost > 0),
    auto_contribute_percent INTEGER NOT NULL DEFAULT 0
        CHECK (auto_contribute_percent BETWEEN 0 AND 100),
    status TEXT NOT NULL CHECK (status IN ('active', 'redeemed', 'cancelled'))
        DEFAULT 'active',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    redeemed_at DATETIME,
    cancelled_at DATETIME
);

CREATE UNIQUE INDEX idx_reward_commitments_one_active
    ON reward_commitments(user_id)
    WHERE status = 'active';

CREATE INDEX idx_reward_commitments_user
    ON reward_commitments(user_id);

-- Widen the point_transactions.reason CHECK to accept the two new ledger
-- reasons used by the commitment flow:
--   * commit_to_goal  - kid moves points from spendable -> committed
--   * goal_break      - parent/kid cancels a commitment, points return
-- SQLite doesn't support ALTER CHECK in place, so we rebuild the table.
PRAGMA foreign_keys = OFF;

CREATE TABLE point_transactions_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount INTEGER NOT NULL,
    reason TEXT NOT NULL CHECK (reason IN (
        'chore_complete', 'chore_uncomplete', 'reward_redeem',
        'streak_bonus', 'admin_adjust', 'expiry_penalty',
        'points_decay', 'missed_chore',
        'commit_to_goal', 'goal_break'
    )),
    reference_id INTEGER,
    note TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO point_transactions_new
    (id, user_id, amount, reason, reference_id, note, idempotency_key, created_at)
SELECT
    id, user_id, amount, reason, reference_id, note, idempotency_key, created_at
FROM point_transactions;

DROP TABLE point_transactions;
ALTER TABLE point_transactions_new RENAME TO point_transactions;

CREATE INDEX idx_point_tx_user ON point_transactions(user_id);
CREATE INDEX idx_point_tx_ref ON point_transactions(reason, reference_id);
CREATE UNIQUE INDEX idx_point_tx_idempotency_key
    ON point_transactions(idempotency_key)
    WHERE idempotency_key IS NOT NULL;

PRAGMA foreign_keys = ON;
