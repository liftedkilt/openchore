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
    side           TEXT    NOT NULL CHECK (side IN ('a', 'b')), -- a made the invite, b joined with it
    hub_url        TEXT    NOT NULL,
    hub_link       TEXT    NOT NULL,
    token          TEXT    NOT NULL,
    secret         TEXT    NOT NULL,
    invite         TEXT    NOT NULL DEFAULT '', -- kept until the other family joins
    invite_expires DATETIME,
    show_pictures  INTEGER NOT NULL DEFAULT 0,  -- show this family's custom animal pictures (off until a parent says so)
    allow_visits   INTEGER NOT NULL DEFAULT 1,  -- take gifts and playdates from this family
    goal_week      TEXT    NOT NULL DEFAULT '', -- the Monday of the week this family's team goal is for ('' = none)
    goal_target    INTEGER NOT NULL DEFAULT 0,  -- the score the two families aim to average together, 1 to 100
    peer_name      TEXT    NOT NULL DEFAULT '',
    snapshot       TEXT    NOT NULL DEFAULT '', -- the other family's last snapshot (JSON)
    snapshot_at    DATETIME,
    last_sync_at   DATETIME,
    last_error     TEXT    NOT NULL DEFAULT '',
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Sharing is chosen per family, in both directions, and a person only shows
-- up when both parents picked them: the sender shares them (friend_link_shares)
-- and the receiver accepts them (friend_link_accepts). Both start empty.
CREATE TABLE friend_link_shares (
    link_id INTEGER NOT NULL REFERENCES friend_links(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (link_id, user_id)
);
-- Gifts and playdates a person here sent to a linked family (they travel in
-- the snapshot for two days), and the ones that arrived from one, each
-- applied once.
CREATE TABLE friend_sends (
    id         TEXT    PRIMARY KEY,
    link_id    INTEGER NOT NULL REFERENCES friend_links(id) ON DELETE CASCADE,
    kind       TEXT    NOT NULL CHECK (kind IN ('gift', 'playdate')),
    from_user  INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    to_member  TEXT    NOT NULL DEFAULT '', -- gift: whose animal, by their snapshot id
    day        TEXT    NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE friend_received (
    link_id     INTEGER NOT NULL REFERENCES friend_links(id) ON DELETE CASCADE,
    send_id     TEXT    NOT NULL,
    kind        TEXT    NOT NULL,
    from_member TEXT    NOT NULL,
    to_user     INTEGER NOT NULL DEFAULT 0, -- gift: whose animal here
    day         TEXT    NOT NULL,           -- the sender's date
    received_on TEXT    NOT NULL,           -- our date
    PRIMARY KEY (link_id, send_id)
);

CREATE TABLE friend_link_accepts (
    link_id   INTEGER NOT NULL REFERENCES friend_links(id) ON DELETE CASCADE,
    member_id TEXT    NOT NULL, -- the person's id in the other family's snapshot
    PRIMARY KEY (link_id, member_id)
);
