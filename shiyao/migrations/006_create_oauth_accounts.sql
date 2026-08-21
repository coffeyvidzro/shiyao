CREATE TABLE IF NOT EXISTS oauth_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL
        REFERENCES users(id)
        ON DELETE CASCADE,

    provider TEXT NOT NULL,

    provider_uid TEXT NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_oauth_accounts_provider
        CHECK (
            provider IN (
                'google',
                'github'
            )
        ),

    CONSTRAINT uq_oauth_accounts_provider_identity
        UNIQUE (provider, provider_uid),

    CONSTRAINT uq_oauth_accounts_user_provider
        UNIQUE (user_id, provider)
);


CREATE INDEX IF NOT EXISTS oauth_accounts_user_id_idx
    ON oauth_accounts (user_id);


CREATE TRIGGER trg_oauth_accounts_set_updated_at
BEFORE UPDATE ON oauth_accounts
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
