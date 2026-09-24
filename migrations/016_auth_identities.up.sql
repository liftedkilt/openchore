-- Server-issued sessions carry the user's session_version; bumping it
-- invalidates every outstanding session for that user (PIN change, unlink,
-- role change, "sign out everywhere").
ALTER TABLE users ADD COLUMN session_version INTEGER NOT NULL DEFAULT 0;

-- External (OIDC) identities linked to an OpenChore profile. A profile may be
-- linked to several providers; a provider subject maps to exactly one profile.
CREATE TABLE user_identities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    subject TEXT NOT NULL,
    email TEXT NOT NULL DEFAULT '',
    display_name TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_login_at DATETIME,
    UNIQUE(provider, subject)
);
CREATE INDEX idx_user_identities_user ON user_identities(user_id);
