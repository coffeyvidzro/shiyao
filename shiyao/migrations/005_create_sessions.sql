CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL
        REFERENCES users(id)
        ON DELETE CASCADE,

    token_hash TEXT NOT NULL UNIQUE,

    ip_address INET,

    user_agent TEXT,

    expires_at TIMESTAMPTZ NOT NULL,

    last_seen_at TIMESTAMPTZ,

    revoked_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_sessions_expiry
        CHECK (expires_at > created_at),

    CONSTRAINT chk_sessions_revoked
        CHECK (
            revoked_at IS NULL
            OR revoked_at >= created_at
        )
);


CREATE INDEX IF NOT EXISTS sessions_user_id_idx
    ON sessions (user_id);


CREATE INDEX IF NOT EXISTS sessions_expires_at_idx
    ON sessions (expires_at);


CREATE INDEX IF NOT EXISTS sessions_active_idx
    ON sessions (user_id)
    WHERE revoked_at IS NULL;
