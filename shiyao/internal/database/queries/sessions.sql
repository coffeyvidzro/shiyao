-- name: CreateSession :one
INSERT INTO sessions (
    user_id,
    token_hash,
    ip_address,
    user_agent,
    expires_at,
    last_seen_at
)
SELECT
    u.id,
    sqlc.arg(token_hash),
    sqlc.narg(ip_address)::inet,
    sqlc.narg(user_agent)::text,
    sqlc.arg(expires_at)::timestamptz,
    NOW()
FROM users AS u
WHERE u.id = sqlc.arg(user_id)
  AND u.disabled_at IS NULL
RETURNING *;


-- name: GetSessionByID :one
SELECT s.*
FROM sessions AS s
JOIN users AS u
    ON u.id = s.user_id
WHERE s.id = sqlc.arg(id)
  AND s.expires_at > NOW()
  AND s.revoked_at IS NULL
  AND u.disabled_at IS NULL
LIMIT 1;


-- name: GetSessionByTokenHash :one
SELECT s.*
FROM sessions AS s
JOIN users AS u
    ON u.id = s.user_id
WHERE s.token_hash = sqlc.arg(token_hash)
  AND s.expires_at > NOW()
  AND s.revoked_at IS NULL
  AND u.disabled_at IS NULL
LIMIT 1;


-- name: ListSessionsByUserID :many
SELECT s.*
FROM sessions AS s
JOIN users AS u
    ON u.id = s.user_id
WHERE s.user_id = sqlc.arg(user_id)
  AND s.expires_at > NOW()
  AND s.revoked_at IS NULL
  AND u.disabled_at IS NULL
ORDER BY s.last_seen_at DESC NULLS LAST;


-- name: RevokeSession :exec
UPDATE sessions
SET revoked_at = NOW()
WHERE id = sqlc.arg(id)
  AND user_id = sqlc.arg(user_id)
  AND revoked_at IS NULL;


-- name: TouchSession :exec
UPDATE sessions
SET last_seen_at = NOW()
WHERE id = sqlc.arg(id)
  AND user_id = sqlc.arg(user_id)
  AND expires_at > NOW()
  AND revoked_at IS NULL
  AND (
      last_seen_at IS NULL
      OR last_seen_at < NOW() - INTERVAL '5 minutes'
  );


-- name: RevokeUserSessions :exec
UPDATE sessions
SET revoked_at = NOW()
WHERE user_id = sqlc.arg(user_id)
  AND revoked_at IS NULL;
