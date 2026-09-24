DROP INDEX IF EXISTS idx_user_identities_user;
DROP TABLE IF EXISTS user_identities;
ALTER TABLE users DROP COLUMN session_version;
