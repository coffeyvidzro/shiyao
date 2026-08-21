-- name: CreateAuthChallenge :one
INSERT INTO auth_challenges (
    auth_transaction_id,
    identifier,
    secret_hash,
    purpose,
    state,
    max_attempts,
    expires_at
)
VALUES (
    sqlc.arg(auth_transaction_id),
    sqlc.arg(identifier),
    sqlc.arg(secret_hash),
    sqlc.arg(purpose),
    sqlc.arg(state),
    sqlc.arg(max_attempts),
    sqlc.arg(expires_at)
)
RETURNING *;


-- name: GetAuthChallengeByID :one
SELECT *
FROM auth_challenges
WHERE id = sqlc.arg(id)
LIMIT 1;


-- name: GetActiveAuthChallenge :one
SELECT *
FROM auth_challenges
WHERE auth_transaction_id = sqlc.arg(auth_transaction_id)
  AND purpose = sqlc.arg(purpose)
  AND consumed_at IS NULL
  AND expires_at > NOW()
  AND attempts < max_attempts
ORDER BY created_at DESC
LIMIT 1;


-- name: IncrementAuthChallengeAttempts :one
UPDATE auth_challenges
SET attempts = attempts + 1
WHERE id = sqlc.arg(id)
  AND consumed_at IS NULL
  AND expires_at > NOW()
  AND attempts < max_attempts
RETURNING *;


-- name: ConsumeAuthChallenge :one
UPDATE auth_challenges
SET consumed_at = NOW()
WHERE id = sqlc.arg(id)
  AND consumed_at IS NULL
  AND expires_at > NOW()
  AND attempts < max_attempts
RETURNING *;


-- name: DeleteExpiredAuthChallenges :exec
DELETE FROM auth_challenges
WHERE expires_at <= NOW()
   OR consumed_at IS NOT NULL;
