-- name: CreateSession :one
INSERT INTO sessions (
    user_id,
    token_hash,
    ip_address,
    user_agent,
    expires_at
)
SELECT
    u.id,
    sqlc.arg(token_hash),
    sqlc.narg(ip_address)::inet,
    sqlc.narg(user_agent)::text,
    sqlc.arg(expires_at)::timestamptz
FROM users AS u
WHERE u.id = sqlc.arg(user_id)
  AND u.disabled_at IS NULL
RETURNING *;
