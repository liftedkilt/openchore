-- Unlinks the hub hasn't confirmed yet. When a parent unlinks a family and
-- the hub can't be reached, the link and the other family's data are still
-- removed here at once, but the hub would keep this family's last snapshot
-- for the other family to read. The hub address and token are kept here
-- until the hub confirms the delete (retried at every sync), for at most
-- 90 days (the hub drops idle links by then anyway).
CREATE TABLE friend_unlinks (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    side       TEXT     NOT NULL CHECK (side IN ('a', 'b')),
    hub_url    TEXT     NOT NULL,
    hub_link   TEXT     NOT NULL,
    token      TEXT     NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
