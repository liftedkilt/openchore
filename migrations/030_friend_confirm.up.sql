-- Add the "pending_confirm" status to friend_links so the inviting parent
-- (side 'a') must confirm the joining family before the link becomes active
-- and starts exchanging member data.
--
-- SQLite can't ALTER a CHECK constraint. Rebuilding friend_links would mean
-- DROP TABLE friend_links, and with foreign keys on that cascades into
-- friend_link_shares, friend_sends, friend_received and friend_link_accepts.
-- Widening a CHECK doesn't change how rows are stored, so edit the stored
-- table definition in place (see 022_optional_category.up.sql).
PRAGMA writable_schema = ON;
UPDATE sqlite_schema
   SET sql = replace(sql,
     'CHECK (status IN (''waiting'', ''active'', ''ended''))',
     'CHECK (status IN (''waiting'', ''pending_confirm'', ''active'', ''ended''))')
 WHERE type = 'table' AND name = 'friend_links';
PRAGMA writable_schema = RESET;
