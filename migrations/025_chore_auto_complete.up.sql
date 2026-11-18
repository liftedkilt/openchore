-- A chore that another app finishes (through the API) when the work is
-- really done, e.g. a learning app reporting that today's practice is
-- complete. Kids can't tick or untick it themselves; parents still can.
ALTER TABLE chores ADD COLUMN auto_complete INTEGER NOT NULL DEFAULT 0;
