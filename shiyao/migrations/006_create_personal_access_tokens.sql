CREATE TABLE IF NOT EXISTS personal_access_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    scopes TEXT[] NOT NULL DEFAULT '{sandbox:read,sandbox:write}',
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pat_user_id
    ON personal_access_tokens(user_id);

CREATE INDEX IF NOT EXISTS idx_pat_prefix
    ON personal_access_tokens(token_prefix);
