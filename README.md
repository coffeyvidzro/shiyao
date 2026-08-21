# Shiyao

**Secure, sub-second microVM execution environment for AI agents.**

Shiyao is a Go control plane for running AI-agent workloads inside isolated
[Firecracker](https://firecracker-microvm.github.io/) microVMs. It is designed
for Linux hosts that can provide KVM, VSOCK, TAP devices, nftables, and cgroup
v2 controls.

## What it provides

- **Authentication** with user sessions and personal access tokens (PATs).
- **Authorization foundation** for separating authenticated identity from
  permissions and credential scopes.
- **Sandbox lifecycle management** from the HTTP control plane through the VMM
  to a running Firecracker microVM.
- **Firecracker lifecycle management** for creating, starting, stopping, and
  snapshot-resuming microVMs.
- **Isolated networking** using one TAP device per guest and a shared nftables
  policy that is default-deny for guest-originated forwarding.
- **VSOCK command execution** with strict request validation, bounded command
  concurrency, output limits, and optional framed stdout/stderr delivery.
- **Admission control** that bounds resident VMs and concurrent provisioning
  operations, returning backpressure instead of accumulating host work.
- **Warm-instance pooling** for reusing prepared VMs only after a caller-supplied
  reset operation succeeds.
- **Snapshot integrity hooks** and a small cold-boot versus snapshot-resume
  measurement utility for integration benchmarks.

## Architecture

```text
AI agent / application
        |
        v
HTTP control plane
        |
        +-- authn --------------------> authenticated principal
        |
        +-- authz --------------------> permission decision
        |
        +-- sandbox service ----------> lifecycle state in Postgres
        |                                  |
        |                                  v
        |                              VMM manager
        |                                  |
        |                                  v
        |                           Firecracker microVM
        |                                  |
        |                                  +-- guest agent over VSOCK
        |
        +-- shared nftables policy --> per-VM TAP interface
```

Authentication and authorization are intentionally separate: `internal/authn`
answers who is authenticated and which credential was used, while `internal/authz`
decides what that principal is allowed to do. Teams and organizations are not
required for the current user + PAT model.

## API surface

The daemon exposes the current API under `/v1`:

```text
POST   /v1/auth/start
POST   /v1/auth/otp
POST   /v1/auth/logout

POST   /v1/tokens
GET    /v1/tokens
DELETE /v1/tokens/:id

GET    /v1/sessions
DELETE /v1/sessions/:id
POST   /v1/sessions/revoke-all

GET    /v1/users/me

GET    /v1/sandboxes
GET    /v1/sandboxes/:id
POST   /v1/sandboxes
DELETE /v1/sandboxes/:id
GET    /v1/sandboxes/:id/exec/stream
```

Authenticated routes accept either the `shiyao-session` cookie or a `Bearer`
personal access token. Sandbox execution requires the authenticated user to
own the sandbox and the sandbox to be in the `running` state before the VSOCK
connection is opened.

## Personal access tokens

PATs are long-lived API credentials for CLI, automation, and agent workloads.
The raw token is returned only when the token is created; Shiyao stores a hash
for later verification and exposes only token metadata and a short prefix after
creation.

The initial scope model includes:

- `sandbox:read`
- `sandbox:write`

Credential authentication produces an `authn.Principal`; authorization remains
a separate concern so the same identity model can later support teams,
organizations, and service identities without changing the basic authn flow.

## Sandbox lifecycle

A sandbox is persisted in Postgres and driven through the VMM lifecycle:

1. The control plane creates a sandbox record in `pending` state.
2. The VMM allocates a guest subnet, VSOCK CID, TAP name, and Firecracker socket
   path.
3. Network resources are configured and Firecracker is configured and started.
4. On success, the sandbox becomes `running` and records its start time.
5. On provisioning failure, the control plane marks the sandbox `failed` while
   the VMM performs its own best-effort cleanup.
6. Deletion stops the VM and releases host resources before removing the
   database record. Cleanup failures leave the sandbox in `cleanup_failed` for
   recovery instead of losing the control-plane record.

## VM lifecycle

1. `Manager.ProvisionVM` obtains a bounded provisioning slot and allocates a
   guest subnet, VSOCK CID, TAP name, and Firecracker socket path.
2. The VMM creates the TAP device and adds the guest's allowed destinations to
   the shared `inet shiyao` nftables sets.
3. Firecracker starts either from a kernel/rootfs configuration or from a
   snapshot resume configuration.
4. The guest agent accepts authenticated VSOCK execution requests.
5. On teardown, Shiyao removes the nftables set elements, TAP device, socket,
   and IPAM/VSOCK allocations.

## Network policy

Guest forwarding is default-deny. The shared nftables forward chain:

- blocks access to `169.254.169.254` (cloud metadata);
- allows established and related connections;
- allows guest DNS to its configured host gateway on UDP port 53;
- allows the configured host proxy and explicitly configured TCP ports on that
  gateway; and
- drops all other traffic originating from a managed TAP interface.

Each VM changes only set elements, rather than creating a dedicated firewall
chain. This keeps TAP/firewall setup small and makes policy updates atomic at
the nftables transaction level.

## VSOCK execution protocol

The host connects to the guest agent on VSOCK port `1024`. The host-side
execution path is exposed through the sandbox WebSocket endpoint and then uses
`internal/vsock.ExecStream` to bridge the request to the guest.

Requests include a protocol version, request ID, command, arguments,
environment allowlist, and optional timeout.

- Requests are capped at 1 MiB.
- Commands are limited to four concurrent executions per guest.
- Captured stdout and stderr are each capped at 10 MiB.
- Requests can opt into newline-delimited output frames. Each frame carries at
  most 64 KiB, followed by one terminal result frame.

The host-side API is `internal/vsock.Exec` for a one-shot result and
`internal/vsock.ExecStream` for framed output delivery.

## Capacity and warm instances

`vmm.ManagerLimits` controls the maximum resident VM count and maximum number
of concurrent provisioning operations. Saturation returns `vmm.ErrBackpressure`
so callers can retry, shed load, or queue work outside the host control plane.

`vmm.WarmPool` manages leases for running, prepared instances. An instance is
never returned to the idle pool until its supplied reset operation succeeds; a
reset failure stops and evicts it to avoid cross-tenant state reuse.

## Configuration

The daemon reads configuration from environment variables (with `.env` loaded
for local development).

Core services:

```text
DATABASE_URL
REDIS_URL
NATS_URL
```

Runtime middleware:

```text
CORS_ORIGINS
DEVELOPMENT
```

Firecracker runtime:

```text
VMM_KERNEL_PATH
VMM_ROOTFS_PATH
VMM_GUEST_AGENT_PATH      # default: /usr/local/bin/shiyao-agent
VMM_VCPU_COUNT            # default: 2
VMM_MEMORY_MB             # default: 512
VMM_BOOT_ARGS             # default: console=ttyS0 reboot=k panic=1 pci=off
VMM_UPLINK_INTERFACE      # default: eth0
```

`VMM_KERNEL_PATH` and `VMM_ROOTFS_PATH` are required for cold-boot execution.
The runtime validates the VMM and network configuration during daemon startup.

## Snapshot configuration

`shiyao.yaml` describes a reusable snapshot image. It defines the base runtime,
language dependencies, resource envelope, environment, and outbound-network
intent used when preparing a VM image.

```yaml
version: "v1alpha1"
name: python-agent
description: Python environment for an AI coding agent

runtime:
    os: linux
    distro: ubuntu-22.04
    architecture: x86_64

language:
    name: python
    version: "3.11"

dependencies:
    system: [curl, git]
    pip: [requests]

resources:
    vcpu: 2
    memory_mb: 512
    disk_mb: 1024

env:
    PYTHONUNBUFFERED: "1"

network:
    allowed_domains: [api.openai.com]
    block_private_ips: true
```

Required resource values are `vcpu`, `memory_mb`, and `disk_mb`; values must
be positive, with at least 128 MiB of memory and a 1 GiB disk. Pin dependency
versions where reproducibility matters. The runtime currently targets Linux
on `x86_64`.

## Benchmarking boot paths

`vmm.MeasureBoots` accepts cold-boot and snapshot-resume operations and returns
their durations. Use it in a privileged integration benchmark with identical
fixtures and a shared guest-readiness check to compare the two paths.

## Requirements

- Go 1.26.6 or later (see `shiyao/go.mod`)
- Linux with KVM access (`/dev/kvm`)
- Firecracker for VM integration runs
- nftables for guest network policy
- cgroup v2 delegation for guest command resource-limit integration tests

## Development

```bash
cd shiyao

test -z "$(gofmt -l .)"
go test -p 1 ./...
go build ./cmd/...
```

Guest sandbox integration tests require root and host kernel capabilities:

```bash
cd shiyao
sudo -E "$(command -v go)" test -tags=integration ./cmd/guest-agent -v -count=1
```

Tests skip cgroup or OverlayFS assertions when the executing environment does
not delegate the necessary kernel capabilities.

## Status

Shiyao is under active development. The control plane, authentication,
networking, VSOCK, admission-control, sandbox lifecycle, and warm-pool
components are implementation foundations. Production deployments should run
privileged integration tests against their target kernel, nftables, Firecracker,
and cgroup configuration before accepting untrusted workloads.

## License

Shiyao is licensed under the [MIT License](LICENSE).
