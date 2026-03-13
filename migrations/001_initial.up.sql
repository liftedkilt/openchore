-- Users
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    avatar_url TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL CHECK (role IN ('admin', 'child')) DEFAULT 'child',
    age INTEGER,
    theme TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- App settings (admin passcode, etc.)
CREATE TABLE app_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO app_settings (key, value) VALUES ('admin_passcode', '0000');

-- Chores
CREATE TABLE chores (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL CHECK (category IN ('required', 'core', 'bonus')) DEFAULT 'core',
    icon TEXT NOT NULL DEFAULT '',
    points_value INTEGER NOT NULL DEFAULT 0,
    estimated_minutes INTEGER,
    source TEXT NOT NULL DEFAULT 'manual',
    external_id TEXT NOT NULL DEFAULT '',
    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Chore schedules
CREATE TABLE chore_schedules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chore_id INTEGER NOT NULL REFERENCES chores(id) ON DELETE CASCADE,
    assigned_to INTEGER NOT NULL REFERENCES users(id),
    assignment_type TEXT NOT NULL CHECK (assignment_type IN ('individual', 'family')) DEFAULT 'individual',
    day_of_week INTEGER CHECK (day_of_week BETWEEN 0 AND 6),
    specific_date DATE,
    available_at TEXT,
    due_by TEXT,
    expiry_penalty TEXT NOT NULL DEFAULT 'block',
    expiry_penalty_value INTEGER NOT NULL DEFAULT 0,
    points_multiplier REAL NOT NULL DEFAULT 1.0,
    start_date DATE,
    end_date DATE,
    recurrence_interval INTEGER,
    recurrence_start DATE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (day_of_week IS NOT NULL OR specific_date IS NOT NULL OR recurrence_interval IS NOT NULL)
);

CREATE INDEX idx_schedules_assigned ON chore_schedules(assigned_to);
CREATE INDEX idx_schedules_chore ON chore_schedules(chore_id);

-- Chore completions
CREATE TABLE chore_completions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chore_schedule_id INTEGER NOT NULL REFERENCES chore_schedules(id) ON DELETE CASCADE,
    completed_by INTEGER NOT NULL REFERENCES users(id),
    status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'rejected')) DEFAULT 'approved',
    photo_url TEXT NOT NULL DEFAULT '',
    approved_by INTEGER REFERENCES users(id),
    approved_at DATETIME,
    completed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completion_date DATE NOT NULL
);

CREATE INDEX idx_completions_schedule_date ON chore_completions(chore_schedule_id, completion_date);
CREATE INDEX idx_completions_completed_by ON chore_completions(completed_by);

-- Points ledger
CREATE TABLE point_transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount INTEGER NOT NULL,
    reason TEXT NOT NULL CHECK (reason IN ('chore_complete', 'chore_uncomplete', 'reward_redeem', 'streak_bonus', 'admin_adjust', 'expiry_penalty', 'points_decay')),
    reference_id INTEGER,
    note TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_point_tx_user ON point_transactions(user_id);
CREATE INDEX idx_point_tx_ref ON point_transactions(reason, reference_id);

-- Rewards store
CREATE TABLE rewards (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    icon TEXT NOT NULL DEFAULT '',
    cost INTEGER NOT NULL CHECK (cost > 0),
    stock INTEGER,
    active INTEGER NOT NULL DEFAULT 1,
    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE reward_redemptions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    reward_id INTEGER NOT NULL REFERENCES rewards(id),
    user_id INTEGER NOT NULL REFERENCES users(id),
    points_spent INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_redemptions_user ON reward_redemptions(user_id);

-- Reward per-user assignments (visibility + custom pricing)
CREATE TABLE reward_assignments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    reward_id INTEGER NOT NULL REFERENCES rewards(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    custom_cost INTEGER,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(reward_id, user_id)
);

CREATE INDEX idx_reward_assignments_reward ON reward_assignments(reward_id);
CREATE INDEX idx_reward_assignments_user ON reward_assignments(user_id);

-- Streaks
CREATE TABLE user_streaks (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    current_streak INTEGER NOT NULL DEFAULT 0,
    longest_streak INTEGER NOT NULL DEFAULT 0,
    streak_start_date DATE,
    last_completed_date DATE,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Streak milestone rewards
CREATE TABLE streak_rewards (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    streak_days INTEGER NOT NULL UNIQUE,
    bonus_points INTEGER NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Webhooks
CREATE TABLE webhooks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL,
    secret TEXT NOT NULL DEFAULT '',
    events TEXT NOT NULL DEFAULT '*',
    active INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Webhook delivery log
CREATE TABLE webhook_deliveries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    webhook_id INTEGER NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event TEXT NOT NULL,
    payload TEXT NOT NULL,
    status_code INTEGER,
    response_body TEXT NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id);
CREATE INDEX idx_webhook_deliveries_created ON webhook_deliveries(created_at);

-- Points decay configuration per user
CREATE TABLE user_decay_config (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    enabled INTEGER NOT NULL DEFAULT 0,
    decay_rate INTEGER NOT NULL DEFAULT 5,
    decay_interval_hours INTEGER NOT NULL DEFAULT 24,
    last_decay_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
