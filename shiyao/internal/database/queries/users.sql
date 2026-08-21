-- name: CreateUser :one
INSERT INTO users (
    name,
    email,
    password_hash
) VALUES (
    sqlc.arg(name),
    sqlc.arg(email),
    sqlc.narg(password_hash)
)
RETURNING *;


-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = sqlc.arg(id)
  AND disabled_at IS NULL
LIMIT 1;


-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = sqlc.arg(email)
  AND disabled_at IS NULL
LIMIT 1;


-- name: GetUserByEmailIncludingDisabled :one
SELECT *
FROM users
WHERE email = sqlc.arg(email)
LIMIT 1;


-- name: SetUserPassword :one
UPDATE users
SET
    password_hash = sqlc.arg(password_hash),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND disabled_at IS NULL
RETURNING *;


-- name: MarkUserEmailVerified :one
UPDATE users
SET
    email_verified = TRUE,
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND disabled_at IS NULL
RETURNING *;


-- name: DisableUser :exec
UPDATE users
SET
    disabled_at = NOW(),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND disabled_at IS NULL;

-- name: UpdateUserProfile :one
UPDATE users
SET
    name = COALESCE(sqlc.narg(name), name),
    email = COALESCE(sqlc.narg(email), email),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND disabled_at IS NULL
RETURNING *;
