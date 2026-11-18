-- Friend families: a weekly leaderboard scored by the share of assigned
-- chore points earned, and links to other families' OpenChore instances so kids can
-- compete and visit each other's farms. See docs/friends.md.

-- One row per person per finished day: chores done out of chores assigned,
-- and the same in the chores' point values (what the score is made from).
-- Paused days are kept as rows with paused = 1 so they stay out of the score
-- after the person is unpaused.
CREATE TABLE score_days (
    user_id  INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date     TEXT    NOT NULL,
    assigned INTEGER NOT NULL DEFAULT 0,
    done     INTEGER NOT NULL DEFAULT 0,
    bonus    INTEGER NOT NULL DEFAULT 0,
    assigned_points INTEGER NOT NULL DEFAULT 0,
    done_points     INTEGER NOT NULL DEFAULT 0,
    paused   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, date)
);

-- A link to another family's OpenChore. The two instances never talk to each
-- other directly: each one calls a hub (cmd/hub), which passes encrypted
-- snapshots between them. token signs this instance in at the hub; secret is
-- the key the two families share, which the hub never sees.
CREATE TABLE friend_links (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    status         TEXT    NOT NULL CHECK (status IN ('waiting', 'active', 'ended')),
    hub_url        TEXT    NOT NULL,
    hub_link       TEXT    NOT NULL,
    token          TEXT    NOT NULL,
    secret         TEXT    NOT NULL,
    invite         TEXT    NOT NULL DEFAULT '', -- kept until the other family joins
    invite_expires DATETIME,
    peer_name      TEXT    NOT NULL DEFAULT '',
    snapshot       TEXT    NOT NULL DEFAULT '', -- the other family's last snapshot (JSON)
    snapshot_at    DATETIME,
    last_sync_at   DATETIME,
    last_error     TEXT    NOT NULL DEFAULT '',
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Nothing about a person leaves the instance until a parent turns this on.
ALTER TABLE users ADD COLUMN share_with_friends INTEGER NOT NULL DEFAULT 0;
