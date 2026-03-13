PRAGMA foreign_keys=OFF;
CREATE TABLE point_transactions_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount INTEGER NOT NULL,
    reason TEXT NOT NULL CHECK (reason IN ('chore_complete', 'chore_uncomplete', 'reward_redeem', 'streak_bonus', 'admin_adjust', 'expiry_penalty', 'points_decay', 'missed_chore')),
    reference_id INTEGER,
    note TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO point_transactions_new SELECT * FROM point_transactions;
DROP TABLE point_transactions;
ALTER TABLE point_transactions_new RENAME TO point_transactions;
PRAGMA foreign_keys=ON;
