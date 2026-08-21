CREATE TABLE IF NOT EXISTS auth_challenges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    auth_transaction_id UUID
        REFERENCES auth_transactions(id)
        ON DELETE CASCADE,

    identifier CITEXT NOT NULL,

    secret_hash TEXT NOT NULL,

    purpose TEXT NOT NULL,

    state JSONB NOT NULL DEFAULT '{}'::jsonb,

    attempts INTEGER NOT NULL DEFAULT 0,

    max_attempts INTEGER NOT NULL DEFAULT 5,

    expires_at TIMESTAMPTZ NOT NULL,

    consumed_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_auth_challenges_purpose
        CHECK (
            purpose IN (
                'email_otp',
                'email_verification',
                'password_reset',
                'magic_link'
            )
        ),

    CONSTRAINT chk_auth_challenges_state
        CHECK (jsonb_typeof(state) = 'object'),

    CONSTRAINT chk_auth_challenges_attempts
        CHECK (attempts >= 0),

    CONSTRAINT chk_auth_challenges_max_attempts
        CHECK (max_attempts > 0),

    CONSTRAINT chk_auth_challenges_expiry
        CHECK (expires_at > created_at),

    CONSTRAINT chk_auth_challenges_consumed
        CHECK (
            consumed_at IS NULL
            OR consumed_at >= created_at
        )
);


CREATE INDEX IF NOT EXISTS auth_challenges_transaction_idx
    ON auth_challenges (auth_transaction_id);


CREATE INDEX IF NOT EXISTS auth_challenges_identifier_idx
    ON auth_challenges (identifier);


CREATE INDEX IF NOT EXISTS auth_challenges_active_idx
    ON auth_challenges (
        identifier,
        purpose,
        expires_at
    )
    WHERE consumed_at IS NULL;


CREATE INDEX IF NOT EXISTS auth_challenges_expires_idx
    ON auth_challenges (expires_at)
    WHERE consumed_at IS NULL;
