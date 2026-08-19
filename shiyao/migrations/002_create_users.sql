CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email citext NOT NULL UNIQUE,
    email_verified BOOLEAN NOT NULL DEFAULT false,
    name text,
    password_hash text,
    disabled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS auth_challenges (
    identifier TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    purpose TEXT NOT NULL,
    state JSONB NOT NULL DEFAULT '{}'::jsonb,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (identifier, token_hash),

    CONSTRAINT chk_auth_challenge_purpose CHECK (
        purpose = 'email_verification' OR
        purpose = 'password_reset' OR
        purpose = 'magic_link'
    ),
    CONSTRAINT chk_auth_challenge_state CHECK (jsonb_typeof(state) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_auth_challenges_expires
    ON auth_challenges (expires_at)
    WHERE consumed_at IS NULL;
