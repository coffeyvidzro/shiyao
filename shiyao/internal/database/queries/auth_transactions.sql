-- name: CreateAuthTransaction :one
INSERT INTO auth_transactions (
    identifier,
    user_id,
    state,
    expires_at
)
VALUES (
    sqlc.arg(identifier),
    sqlc.narg(user_id),
    'started',
    sqlc.arg(expires_at)
)
RETURNING *;


-- name: GetAuthTransactionByID :one
SELECT *
FROM auth_transactions
WHERE id = sqlc.arg(id)
  AND expires_at > NOW()
  AND state NOT IN ('authenticated', 'expired')
LIMIT 1;


-- name: GetAuthTransactionByIDAnyState :one
SELECT *
FROM auth_transactions
WHERE id = sqlc.arg(id)
LIMIT 1;


-- name: SetAuthTransactionMethod :one
UPDATE auth_transactions
SET
    selected_method = sqlc.arg(selected_method),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND expires_at > NOW()
  AND state NOT IN ('authenticated', 'expired')
RETURNING *;


-- name: SetAuthTransactionState :one
UPDATE auth_transactions
SET
    state = sqlc.arg(state),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND expires_at > NOW()
  AND state NOT IN ('authenticated', 'expired')
RETURNING *;


-- name: SetAuthTransactionUser :one
UPDATE auth_transactions
SET
    user_id = sqlc.arg(user_id),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND expires_at > NOW()
RETURNING *;


-- name: MarkAuthTransactionAuthenticated :one
UPDATE auth_transactions
SET
    state = 'authenticated',
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND expires_at > NOW()
  AND state <> 'expired'
RETURNING *;


-- name: ExpireAuthTransaction :exec
UPDATE auth_transactions
SET
    state = 'expired',
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND state NOT IN ('authenticated', 'expired');


-- name: DeleteExpiredAuthTransactions :exec
DELETE FROM auth_transactions
WHERE expires_at <= NOW();
