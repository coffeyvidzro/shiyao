CREATE TABLE IF NOT EXISTS auth_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    identifier CITEXT NOT NULL,

    user_id UUID REFERENCES users(id) ON DELETE CASCADE,

    state TEXT NOT NULL DEFAULT 'started',

    selected_method TEXT,

    expires_at TIMESTAMPTZ NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_auth_transactions_state
        CHECK (
            state IN (
                'started',
                'otp_sent',
                'otp_verified',
                'password_required',
                'authenticated',
                'expired'
            )
        ),

    CONSTRAINT chk_auth_transactions_method
        CHECK (
            selected_method IS NULL
            OR selected_method IN (
                'otp',
                'password',
                'google',
                'github'
            )
        ),

    CONSTRAINT chk_auth_transactions_expiry
        CHECK (expires_at > created_at)
);


CREATE INDEX IF NOT EXISTS auth_transactions_identifier_idx
    ON auth_transactions (identifier);


CREATE INDEX IF NOT EXISTS auth_transactions_user_id_idx
    ON auth_transactions (user_id);


CREATE INDEX IF NOT EXISTS auth_transactions_active_idx
    ON auth_transactions (identifier, expires_at)
    WHERE state NOT IN ('authenticated', 'expired');


CREATE TRIGGER trg_auth_transactions_set_updated_at
BEFORE UPDATE ON auth_transactions
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
