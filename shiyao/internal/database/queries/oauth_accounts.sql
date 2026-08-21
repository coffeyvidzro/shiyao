-- name: CreateOAuthAccount :one
INSERT INTO oauth_accounts (
    user_id,
    provider,
    provider_uid
)
VALUES (
    sqlc.arg(user_id),
    sqlc.arg(provider),
    sqlc.arg(provider_uid)
)
RETURNING *;


-- name: GetOAuthAccount :one
SELECT *
FROM oauth_accounts
WHERE provider = sqlc.arg(provider)
  AND provider_uid = sqlc.arg(provider_uid)
LIMIT 1;


-- name: GetOAuthAccountByID :one
SELECT *
FROM oauth_accounts
WHERE id = sqlc.arg(id)
LIMIT 1;


-- name: ListOAuthAccountsByUserID :many
SELECT *
FROM oauth_accounts
WHERE user_id = sqlc.arg(user_id)
ORDER BY created_at ASC;


-- name: GetUserOAuthAccount :one
SELECT *
FROM oauth_accounts
WHERE user_id = sqlc.arg(user_id)
  AND provider = sqlc.arg(provider)
LIMIT 1;


-- name: DeleteOAuthAccount :exec
DELETE FROM oauth_accounts
WHERE id = sqlc.arg(id)
  AND user_id = sqlc.arg(user_id);


-- name: UpdateOAuthAccount :one
UPDATE oauth_accounts
SET
    provider_uid = sqlc.arg(provider_uid),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND user_id = sqlc.arg(user_id)
RETURNING *;
