-- name: CreateSandbox :one
INSERT INTO sandboxes (
    user_id,
    vm_id,
    template,
    vcpu,
    memory_mb,
    timeout_seconds,
    allowed_hosts,
    status
) VALUES (
    sqlc.arg(user_id),
    sqlc.arg(vm_id),
    sqlc.arg(template),
    sqlc.arg(vcpu),
    sqlc.arg(memory_mb),
    sqlc.arg(timeout_seconds),
    sqlc.arg(allowed_hosts),
    sqlc.narg(status)
)
RETURNING *;

-- name: GetSandboxByID :one
SELECT * FROM sandboxes
WHERE id = sqlc.arg(id)
LIMIT 1;

-- name: GetSandboxByVMID :one
SELECT * FROM sandboxes
WHERE vm_id = sqlc.arg(vm_id)
LIMIT 1;

-- name: UpdateSandboxStatus :one
UPDATE sandboxes
SET
    status = sqlc.arg(status),
    started_at = CASE WHEN sqlc.arg(status) = 'running' THEN NOW() ELSE started_at END,
    stopped_at = CASE WHEN sqlc.arg(status) IN ('stopped', 'failed', 'cleanup_failed') THEN NOW() ELSE stopped_at END
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ListSandboxesByUser :many
SELECT * FROM sandboxes
WHERE user_id = sqlc.arg(user_id)
ORDER BY created_at DESC;

-- name: FindStuckSandboxes :many
SELECT * FROM sandboxes
WHERE status = sqlc.arg(status)
  AND created_at < NOW() - (sqlc.arg(max_age_seconds)::text || ' seconds')::interval
ORDER BY created_at ASC;

-- name: DeleteSandbox :exec
DELETE FROM sandboxes WHERE id = sqlc.arg(id);
