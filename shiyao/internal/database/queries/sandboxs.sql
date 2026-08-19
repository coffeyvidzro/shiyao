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

