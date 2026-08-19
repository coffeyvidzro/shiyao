CREATE TABLE IF NOT EXISTS sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    ip_address inet,
    user_agent text,
    expires_at timestamptz NOT NULL,
    last_seen_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT chk_sessions_expiry CHECK (expires_at > created_at)
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);
CREATE INDEX idx_sessions_active ON sessions (user_id) WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS oauth_accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider text NOT NULL,
    provider_uid text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT chk_oauth_accounts_provider CHECK (provider IN ('google', 'github')),
    CONSTRAINT uq_oauth_accounts_provider_identity UNIQUE (provider, provider_uid),
    CONSTRAINT uq_oauth_accounts_user_provider UNIQUE (user_id, provider)
);

CREATE INDEX oauth_accounts_user_id_idx ON oauth_accounts (user_id);

CREATE TRIGGER trg_oauth_accounts_set_updated_at
BEFORE UPDATE ON oauth_accounts
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
