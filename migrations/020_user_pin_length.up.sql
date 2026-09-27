-- How many digits a profile's PIN has, so the PIN pad can show the right
-- number of dots and submit as soon as the last one is entered. 0 means
-- unknown: PINs set before this column existed are only stored as a hash,
-- so their length is recorded the next time they are used to sign in.
ALTER TABLE users ADD COLUMN pin_length INTEGER NOT NULL DEFAULT 0;
