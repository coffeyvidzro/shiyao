-- name: CreateToken :one
INSERT INTO personal_access_tokens (
    user_id,
    name,
    token_hash,
    token_prefix,
    scopes,
    expires_at
) VALUES (
    sqlc.arg(user_id),
    sqlc.arg(name),
    sqlc.arg(token_hash),
    sqlc.arg(token_prefix),
    sqlc.arg(scopes),
    sqlc.narg(expires_at)
)
RETURNING *;

-- name: GetTokenByHash :one
SELECT *
FROM personal_access_tokens
WHERE token_hash = sqlc.arg(token_hash)
  AND (expires_at IS NULL OR expires_at > NOW());

-- name: ListTokensByUser :many
SELECT id, name, token_prefix, scopes, expires_at, last_used_at, created_at
FROM personal_access_tokens
WHERE user_id = sqlc.arg(user_id)
ORDER BY created_at DESC;

-- name: RevokeToken :exec
DELETE FROM personal_access_tokens
WHERE id = sqlc.arg(id)
  AND user_id = sqlc.arg(user_id);

-- name: TouchToken :exec
UPDATE personal_access_tokens
SET last_used_at = NOW()
WHERE id = sqlc.arg(id)
  AND (last_used_at IS NULL
       OR last_used_at < NOW() - INTERVAL '1 minute');
