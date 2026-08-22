-- name: CreateTeamToken :one
INSERT INTO team_access_tokens (
    team_id,
    name,
    token_hash,
    token_prefix,
    scopes,
    expires_at,
    created_by
)
SELECT
    sqlc.arg(team_id),
    sqlc.arg(name),
    sqlc.arg(token_hash),
    sqlc.arg(token_prefix),
    sqlc.arg(scopes),
    sqlc.narg(expires_at),
    sqlc.arg(created_by)
WHERE EXISTS (
    SELECT 1
    FROM team_members
    WHERE team_id = sqlc.arg(team_id)
      AND user_id = sqlc.arg(created_by)
      AND role IN ('owner', 'admin')
)
RETURNING *;

-- name: GetTeamTokenByHash :one
SELECT *
FROM team_access_tokens
WHERE token_hash = sqlc.arg(token_hash)
  AND (expires_at IS NULL OR expires_at > NOW());

-- name: ListTeamTokens :many
SELECT tat.id, tat.name, tat.token_prefix, tat.scopes, tat.expires_at, tat.last_used_at, tat.created_by, tat.created_at
FROM team_access_tokens AS tat
WHERE tat.team_id = sqlc.arg(team_id)
  AND EXISTS (
      SELECT 1
      FROM team_members
      WHERE team_id = tat.team_id
        AND user_id = sqlc.arg(user_id)
  )
ORDER BY tat.created_at DESC;

-- name: RevokeTeamToken :execrows
DELETE FROM team_access_tokens AS tat
WHERE tat.id = sqlc.arg(id)
  AND tat.team_id = sqlc.arg(team_id)
  AND EXISTS (
      SELECT 1
      FROM team_members
      WHERE team_id = tat.team_id
        AND user_id = sqlc.arg(user_id)
        AND role IN ('owner', 'admin')
  );

-- name: TouchTeamToken :exec
UPDATE team_access_tokens
SET last_used_at = NOW()
WHERE id = sqlc.arg(id)
  AND (last_used_at IS NULL
       OR last_used_at < NOW() - INTERVAL '1 minute');
