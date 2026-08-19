# Contributing to Shiyao

Thanks for helping improve Shiyao. The project is still early, so small, well-tested changes are easiest to review.

## Development setup

Shiyao is a Go service backed by PostgreSQL, Redis, NATS JetStream, and Firecracker. Firecracker work must be tested on Linux with KVM access.

```bash
docker compose -f deploy/compose.yaml up -d postgres redis nats caddy
cd shiyao
go test ./...
```

Useful local service URLs:

- PostgreSQL: `postgres://shiyao:shiyao_dev_password@localhost:5432/shiyao?sslmode=disable`
- Redis: `redis://localhost:6379`
- NATS: `nats://shiyao:shiyao_dev_password@localhost:4222`
- Caddy: `http://localhost`

## Pull requests

Before opening a pull request:

1. Keep changes focused and describe the user-visible behavior.
2. Run `gofmt` on touched Go files.
3. Run `go test ./...` from `shiyao/`.
4. For Firecracker changes, document the Linux/KVM environment used for manual testing.
5. Do not commit secrets, generated credentials, VM images, kernels, or large rootfs artifacts.

## Code guidelines

- Prefer small packages with explicit dependencies.
- Return wrapped errors with context.
- Keep comments useful and avoid narrating obvious control flow.
- Treat VM, networking, and sandbox-boundary changes as security-sensitive.

## Local infrastructure

The Compose file in `deploy/compose.yaml` starts only shared development infrastructure. Run the daemon and worker from your checkout while developing so code changes do not require rebuilding containers.
