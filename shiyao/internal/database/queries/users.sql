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
SELECT * FROM users
WHERE id = sqlc.arg(id)
LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = sqlc.arg(email)
LIMIT 1;
