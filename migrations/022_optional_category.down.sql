-- Optional chores become Bonus chores (they also never counted toward the
-- day), then the CHECK goes back to required/core/bonus. See the up
-- migration for why the definition is edited in place.
UPDATE chores SET category = 'bonus' WHERE category = 'optional';
PRAGMA writable_schema = ON;
UPDATE sqlite_schema
   SET sql = replace(sql, 'CHECK (category IN (''required'', ''core'', ''bonus'', ''optional''))', 'CHECK (category IN (''required'', ''core'', ''bonus''))')
 WHERE type = 'table' AND name = 'chores';
PRAGMA writable_schema = RESET;
