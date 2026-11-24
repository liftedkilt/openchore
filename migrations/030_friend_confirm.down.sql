-- Any unconfirmed link reverts to waiting (never active, so a downgrade never
-- silently opens an unconfirmed link; clear any cached snapshot just in case),
-- then narrow the CHECK back. See the up migration for why the table
-- definition is edited in place.
UPDATE friend_links
   SET status = 'waiting', snapshot = '', snapshot_at = NULL
 WHERE status = 'pending_confirm';
PRAGMA writable_schema = ON;
UPDATE sqlite_schema
   SET sql = replace(sql,
     'CHECK (status IN (''waiting'', ''pending_confirm'', ''active'', ''ended''))',
     'CHECK (status IN (''waiting'', ''active'', ''ended''))')
 WHERE type = 'table' AND name = 'friend_links';
PRAGMA writable_schema = RESET;
