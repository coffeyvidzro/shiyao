CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    email CITEXT NOT NULL UNIQUE,

    email_verified BOOLEAN NOT NULL DEFAULT false,

    name TEXT,

    password_hash TEXT,

    disabled_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);


CREATE TRIGGER trg_users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();


CREATE INDEX IF NOT EXISTS users_email_verified_idx
    ON users (email_verified)
    WHERE email_verified = true;


CREATE INDEX IF NOT EXISTS users_active_idx
    ON users (id)
    WHERE disabled_at IS NULL;
