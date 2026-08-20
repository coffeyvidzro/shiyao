# Shiyao

Shiyao is a Go control plane for running AI-agent workloads inside isolated
[Firecracker](https://firecracker-microvm.github.io/) microVMs. It is designed
for Linux hosts that can provide KVM, VSOCK, TAP devices, nftables, and cgroup
v2 controls.

## What it provides

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
Shiyao control plane
        |
        +-- VMM admission gate ----> Firecracker microVM
        |                                  |
        |                                  +-- guest agent over VSOCK
        |
        +-- shared nftables policy --> per-VM TAP interface
```

### VM lifecycle

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

The host connects to the guest agent on VSOCK port `1024`. Requests include a
protocol version, request ID, command, arguments, environment allowlist, and
optional timeout.

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
# Run formatting and unit tests.
cd shiyao
test -z "$(gofmt -l .)"
go test -p 1 ./...

# Build all command binaries.
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

Shiyao is under active development. The networking, VSOCK, admission-control,
and warm-pool components are implementation foundations; production deployments
should run privileged integration tests against their target kernel, nftables,
Firecracker, and cgroup configuration before accepting untrusted workloads.
