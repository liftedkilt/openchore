-- Add the "optional" chore category: chores that don't count toward the day
-- (no streak, missed-chore penalty, decay or bonus gate) and may be worth
-- points or nothing. Kids can add their own optional chores.
--
-- SQLite can't ALTER a CHECK constraint. Rebuilding `chores` would mean
-- DROP TABLE chores, and with foreign keys on that cascades into every
-- schedule and completion. Widening a CHECK doesn't change how rows are
-- stored, so SQLite's documented alternative applies: edit the stored
-- table definition in place.
PRAGMA writable_schema = ON;
UPDATE sqlite_schema
   SET sql = replace(sql, 'CHECK (category IN (''required'', ''core'', ''bonus''))', 'CHECK (category IN (''required'', ''core'', ''bonus'', ''optional''))')
 WHERE type = 'table' AND name = 'chores';
PRAGMA writable_schema = RESET;
