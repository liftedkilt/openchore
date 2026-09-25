-- OIDC providers added from Manage -> Settings. Providers from auth.oidc in
-- config.yaml or the OIDC_* environment variables are not stored here; they
-- are read on every start and take precedence over a row with the same id.
CREATE TABLE oidc_providers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    issuer TEXT NOT NULL,
    client_id TEXT NOT NULL,
    client_secret TEXT NOT NULL DEFAULT '',
    scopes TEXT NOT NULL DEFAULT '',
    prompt TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
