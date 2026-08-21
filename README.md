# Shiyao

A Go control plane for running AI-agent workloads inside isolated Firecracker microVMs.

Shiyao (石爻) manages Firecracker microVMs on Linux hosts with KVM, VSOCK, TAP devices, nftables, and cgroup v2 support.

## Overview

Shiyao provides infrastructure for executing untrusted code in isolated microVMs with strict network and resource controls.

### Core Components

- **VMM Management**: Firecracker lifecycle management (create, start, stop, snapshot-resume)
- **Network Isolation**: Per-VM TAP devices with shared nftables policy (default-deny forwarding)
- **VSOCK Protocol**: Command execution with request validation, bounded concurrency, output limits, and optional framed stdout/stderr
- **Admission Control**: Bounds on resident VMs and concurrent provisioning operations
- **Warm Pooling**: Reuse of prepared VM instances with caller-supplied reset operations
- **Snapshot Support**: Integrity verification and cold-boot vs snapshot-resume benchmarking

## Architecture

```
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

### VM Lifecycle

1. `Manager.ProvisionVM` obtains a provisioning slot and allocates guest subnet, VSOCK CID, TAP name, and socket path
2. VMM creates TAP device and adds guest's allowed destinations to shared `inet shiyao` nftables sets
3. Firecracker starts from kernel/rootfs configuration or snapshot resume configuration
4. Guest agent accepts authenticated VSOCK execution requests
5. On teardown, nftables set elements, TAP device, socket, and IPAM/VSOCK allocations are removed

### Network Policy

Guest forwarding is default-deny. The shared nftables forward chain:
- Blocks access to `169.254.169.254` (cloud metadata)
- Allows established and related connections
- Allows guest DNS to configured host gateway on UDP port 53
- Allows configured host proxy and explicitly configured TCP ports on that gateway
- Drops all other traffic originating from managed TAP interfaces

Each VM modifies set elements rather than creating dedicated firewall chains. Policy updates are atomic at the nftables transaction level.

### VSOCK Execution Protocol

The host connects to the guest agent on VSOCK port `1024`. Requests include protocol version, request ID, command, arguments, environment allowlist, and optional timeout.

Constraints:
- Requests capped at 1 MiB
- Maximum four concurrent executions per guest
- Captured stdout/stderr capped at 10 MiB each
- Optional newline-delimited output frames (max 64 KiB per frame), followed by terminal result frame

Host-side API: `internal/vsock.Exec` (one-shot result) and `internal/vsock.ExecStream` (framed output delivery).

### Capacity and Warm Instances

`vmm.ManagerLimits` controls maximum resident VM count and concurrent provisioning operations. Saturation returns `vmm.ErrBackpressure`.

`vmm.WarmPool` manages leases for running, prepared instances. Instances are not returned to the idle pool until the supplied reset operation succeeds. Reset failures stop and evict the instance.

### Benchmarking

`vmm.MeasureBoots` accepts cold-boot and snapshot-resume operations and returns their durations. Use in privileged integration benchmarks with identical fixtures and shared guest-readiness checks.

## Requirements

- Go 1.26.6 or later (see `shiyao/go.mod`)
- Linux with KVM access (`/dev/kvm`)
- Firecracker for VM integration runs
- nftables for guest network policy
- cgroup v2 delegation for guest command resource-limit integration tests

## Development

```bash
# Run formatting and unit tests
cd shiyao
test -z "$(gofmt -l .)"
go test -p 1 ./...

# Build all command binaries
go build ./cmd/...
```

Guest sandbox integration tests require root and host kernel capabilities:

```bash
cd shiyao
sudo -E "$(command -v go)" test -tags=integration ./cmd/guest-agent -v -count=1
```

Tests skip cgroup or OverlayFS assertions when the executing environment does not delegate the necessary kernel capabilities.

## Configuration

`shiyao.yaml` is the declarative description of a reusable snapshot image. It defines the base runtime, language dependencies, resource envelope, environment, and outbound-network intent used when preparing a VM image.

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

Required resource values: `vcpu`, `memory_mb`, and `disk_mb` (positive values, minimum 128 MiB memory and 1 GiB disk). Pin dependency versions where reproducibility matters. The runtime currently targets Linux on `x86_64`.

## Status

Shiyao is under active development. The networking, VSOCK, admission-control, and warm-pool components are implementation foundations. Production deployments should run privileged integration tests against their target kernel, nftables, Firecracker, and cgroup configuration before accepting untrusted workloads.

## License

Shiyao is licensed under the [MIT License](LICENSE).