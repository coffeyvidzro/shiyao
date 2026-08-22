# Contributing to Shiyao

Thanks for helping improve Shiyao. The project is still early, so small, well-tested changes are easiest to review.

## Before you start

Shiyao is a **Linux-only** project that manages Firecracker microVMs. You will need:

- A Linux machine (or VM/WSL2) with **KVM access** (`/dev/kvm` must exist)
- Go 1.26+
- Docker (for Postgres, Redis, NATS)
- `nftables` and `cgroup v2` enabled on your kernel

Verify KVM works:

```bash
ls -l /dev/kvm
# Should show: crw-rw-rw- 1 root kvm 10, 232 ... /dev/kvm
```

If you don't have KVM access, you can still contribute to the API layer, database queries, SDK, and documentation — just skip the Firecracker integration tests.

## Architecture overview

Shiyao has two binaries that run separately:

| Binary       | Role                                            | Privileges           | Network             |
| ------------ | ----------------------------------------------- | -------------------- | ------------------- |
| `cmd/daemon` | Public API server (Gin HTTP + WebSocket)        | Unprivileged         | Exposed to internet |
| `cmd/worker` | Hypervisor manager (Firecracker, TAP, nftables) | Root / CAP_NET_ADMIN | Internal only       |

They communicate via **NATS JetStream**. The daemon publishes events (e.g., `sandbox.create`), and the worker consumes them to provision VMs.

Key packages:

- `internal/vmm/` — Firecracker lifecycle, state machine, warm pool, snapshots
- `internal/network/` — IPAM, TAP devices, nftables policy
- `internal/vsock/` — Host ↔ guest agent protocol
- `internal/platform/sandbox/` — HTTP API handlers and business logic
- `cmd/guest-agent/` — Runs as PID 1 inside the microVM

## Development setup

```bash
# 1. Start dependencies
docker compose -f deploy/compose.yaml up -d postgres redis nats caddy

# 2. Run migrations
cd shiyao
atlas migrate apply --url "postgres://shiyao:shiyao_dev_password@localhost:5432/shiyao?sslmode=disable"

# 3. Regenerate sqlc (if you changed any .sql files)
sqlc generate

# 4. Run unit tests (no KVM required)
go test ./...

# 5. Run integration tests (requires KVM + root)
sudo -E go test -tags=integration ./cmd/guest-agent -v -count=1
```

Useful local service URLs:

- PostgreSQL: `postgres://shiyao:shiyao_dev_password@localhost:5432/shiyao?sslmode=disable`
- Redis: `redis://localhost:6379`
- NATS: `nats://shiyao:shiyao_dev_password@localhost:4222`
- Caddy: `http://localhost`

## File naming convention

Each domain module (e.g., `internal/platform/sandbox/`, `internal/identity/users/`) follows a strict layout:

| File            | Purpose                               |
| --------------- | ------------------------------------- |
| `model.go`      | Domain models, request/response types |
| `repository.go` | Database queries (wraps `sqlc`)       |
| `service.go`    | Business logic, orchestration         |
| `validation.go` | Input validation                      |
| `handler.go`    | HTTP handlers (Gin)                   |
| `routes.go`     | Route registration                    |
| `consumer.go`   | NATS event consumers                  |
| `jobs.go`       | Background/recurring work             |

Stick to this layout. It keeps the codebase navigable.

## Pull requests

Before opening a PR:

1. Keep changes focused — one feature or fix per PR.
2. Run `gofmt` on touched Go files.
3. Run `go test ./...` from `shiyao/`.
4. For Firecracker/networking changes, document the Linux/KVM environment used for testing.
5. Do **not** commit secrets, VM images, kernels, or large rootfs artifacts.
6. If you changed `.sql` files, run `sqlc generate` and commit the output.
7. If you added a migration, run `atlas migrate hash` and commit `atlas.sum`.

## Code guidelines

- Prefer small packages with explicit dependencies.
- Return wrapped errors with context (`fmt.Errorf("do thing: %w", err)`).
- Keep comments useful; don't narrate obvious control flow.
- Treat VM, networking, and sandbox-boundary changes as **security-sensitive** — these get extra scrutiny.
- New VSOCK protocol changes must update `internal/vsock/limits.go` and be reviewed for resource exhaustion.

## Where help is most needed

These areas have the highest impact right now:

- **Python SDK** — Most AI developers use Python; we need a clean async client
- **TypeScript SDK** — For web developers building agent UIs
- **Integration tests** — Especially for Firecracker boot/execute/destroy flows
- **Framework integrations** — LangChain, CrewAI, AutoGen tools that wrap Shiyao
- **ARM64 guest support** — Enable Apple Silicon and AWS Graviton users
- **Documentation** — Tutorials, architecture diagrams, deployment guides

Look for issues labeled [`good first issue`](https://github.com/coffeyvidzro/shiyao/issues?q=is%3Aissue+label%3A%22good+first+issue%22) if you're new to the project.

## Security

If you find a security vulnerability, **do not open a public issue**. Email `shiyao@vidzro.com` instead. See [SECURITY.md](SECURITY.md) for details.

## Questions?

Open a GitHub Discussion or reach out in the project Discord. We're happy to help you get set up.
