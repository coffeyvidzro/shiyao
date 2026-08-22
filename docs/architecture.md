# Architecture

Shiyao (石爻) is a self-hosted execution platform for running untrusted AI agent code in isolated Firecracker microVMs.

The architecture is built around a simple security boundary:

```text
┌───────────────────────────────┐
│       Shiyao control plane    │
│                               │
│  API / identity / lifecycle   │
│  admission / authorization    │
└───────────────┬───────────────┘
                │
                ▼
┌───────────────────────────────┐
│      VMM / execution layer    │
│                               │
│  Firecracker  ·  IPAM         │
│  TAP  ·  nftables  ·  VSOCK  │
└───────────────┬───────────────┘
                │
                ▼
┌───────────────────────────────┐
│       Untrusted guest         │
│                               │
│       guest-agent             │
│       PID 1 / VSOCK           │
│       cgroup / rlimits        │
└───────────────────────────────┘
```

The control plane is responsible for deciding **what** should run and **which resources** it may use. The VM/execution layer is responsible for creating and managing the isolation boundary. The guest is treated as untrusted.

## Components

### VMM manager

The VMM manager owns the lifecycle of Firecracker instances and coordinates the resources required by each VM.

Its responsibilities include:

- VM creation, start, stop, and deletion.
- Admission control and backpressure.
- VSOCK CID allocation.
- Guest subnet/IPAM allocation.
- TAP device lifecycle.
- Network-policy setup and cleanup.
- Snapshot resume.
- Warm-instance pooling.
- Coordinated cleanup when a VM is destroyed.

The manager keeps live instance state in memory and coordinates resource ownership across the VMM, network, IPAM, and VSOCK subsystems.

### Firecracker

Firecracker provides the microVM isolation boundary. Shiyao configures each VM with a kernel, root filesystem, vCPU and memory resources, networking, and a VSOCK device.

Shiyao supports both normal VM boot and snapshot resume. Snapshot support is intended to reduce startup work for prepared environments; actual latency depends on the host and guest configuration.

### Network and IPAM

Each VM receives its own TAP interface and guest network allocation.

The network layer is responsible for:

```text
VM
 │
 │ VirtIO network
 ▼
TAP device
 │
 ▼
nftables policy
 │
 ▼
host network
```

Guest forwarding is default-deny. Shiyao maintains shared nftables policy using sets rather than creating an independent firewall chain for every VM.

The current policy includes controls for:

- Established/related connections.
- Guest DNS to the configured host gateway.
- Configured proxy/host-gateway access.
- Explicitly permitted outbound TCP destinations/ports.
- Blocking the cloud metadata address `169.254.169.254`.
- Dropping other traffic originating from managed guest TAP interfaces.

Network resources are allocated before the VM becomes runnable and released during normal teardown.

## VSOCK execution boundary

The host communicates with the guest agent through Firecracker VSOCK rather than exposing an SSH or guest-network execution service.

The guest agent listens on VSOCK port `1024` and accepts a bounded execution protocol.

At the host side, the execution API provides:

- `Exec` for a bounded request/response execution.
- `ExecStream` for framed stdout/stderr streaming followed by a terminal result.

The protocol validates request sizes and fields and applies execution limits including:

- Maximum request size: **1 MiB**.
- Maximum concurrent executions per guest: **4**.
- Maximum captured stdout: **10 MiB**.
- Maximum captured stderr: **10 MiB**.
- Maximum streaming frame: **64 KiB**.
- Bounded command execution timeout.

VSOCK provides the transport boundary, not a blanket trust boundary. Guest-side execution must still be treated as untrusted, and the host should validate the identity and lifecycle of the VM associated with an execution request.

## Guest agent

The guest agent is a small Go binary that runs inside the microVM and provides the execution endpoint.

It is responsible for:

- Receiving and validating execution requests.
- Executing commands.
- Applying guest-side resource controls.
- Returning bounded stdout/stderr and exit information.
- Managing the guest's ephemeral execution environment.

Guest-side resource controls use Linux cgroup v2 and `rlimit` mechanisms where the host/guest environment provides the required capabilities.

The guest filesystem is designed to be ephemeral using OverlayFS/PivotRoot-based setup rather than treating the workload filesystem as durable state.

## VM lifecycle

A normal VM lifecycle is coordinated by the VMM manager:

```text
             ┌─────────────┐
             │   request   │
             └──────┬──────┘
                    ▼
             ┌─────────────┐
             │   admission │
             └──────┬──────┘
                    ▼
        ┌────────────────────────┐
        │ allocate VM resources  │
        │ CID / IP / TAP / socket│
        └───────────┬────────────┘
                    ▼
             ┌─────────────┐
             │ configure   │
             │ Firecracker │
             └──────┬──────┘
                    ▼
          ┌────────────────────┐
          │ cold boot OR       │
          │ snapshot resume    │
          └─────────┬──────────┘
                    ▼
             ┌─────────────┐
             │   running   │
             └──────┬──────┘
                    │
              VSOCK exec
                    │
                    ▼
             ┌─────────────┐
             │   teardown  │
             └──────┬──────┘
                    ▼
        ┌────────────────────────┐
        │ release network, IPAM, │
        │ VSOCK and VM resources │
        └────────────────────────┘
```

Admission control happens before expensive provisioning work. If the configured capacity is exhausted, callers receive backpressure instead of creating an unbounded number of VMs.

Cleanup is treated as part of the lifecycle rather than as an optional best-effort operation: TAP devices, nftables state, IPAM allocations, VSOCK allocations, and Firecracker resources all have to be accounted for when a VM is removed.

## Warm instances

Shiyao can keep prepared VMs in a warm pool to avoid repeated provisioning work.

A warm instance is leased to a caller and is only returned to the idle pool after the caller-provided reset operation succeeds. If reset fails, the instance is stopped and evicted instead of being returned to the pool.

This is an important security boundary:

```text
untrusted workload
       │
       ▼
   warm VM
       │
       ▼
     reset
       │
   ┌───┴────┐
   │        │
success   failure
   │        │
   ▼        ▼
 reuse    destroy
```

Warm pooling therefore depends on the reset procedure establishing the intended clean state. It should not be treated as equivalent to destroying and recreating a VM unless that guarantee has been demonstrated for the deployed guest image and workload model.

## Snapshots

Snapshots provide another fast-start path. A prepared VM can be resumed rather than performing a complete cold boot and guest initialization sequence.

Shiyao includes benchmark support for comparing cold boot and snapshot-resume durations. These measurements should use the same guest image and readiness criteria and should be run as privileged integration benchmarks on the target host configuration.

The architecture intentionally does not assume a universal startup number: startup time depends on Firecracker, the Linux host, guest image, snapshot state, and what the caller considers “ready.”

## Security boundaries

Shiyao treats the guest workload as hostile code.

The important boundaries are:

### Host ↔ guest

The host communicates with the guest through VSOCK and controls the VM lifecycle. The guest should not be trusted with host filesystem, process, or control-plane privileges.

### Guest ↔ network

Guest traffic passes through a dedicated TAP interface and host-side nftables policy. Forwarding is default-deny, with explicit exceptions for allowed traffic.

### Workload ↔ workload

Each workload runs inside its own microVM. Network and execution resources are associated with the VM rather than shared directly between guest processes and other VMs.

### Control plane ↔ capacity

Admission limits prevent callers from consuming unlimited VM slots or provisioning concurrency. Resource configuration provides the per-VM execution envelope.

These are defense-in-depth controls, not a claim that every possible host or hypervisor vulnerability is eliminated. A production deployment should validate the exact Linux kernel, KVM, Firecracker, nftables, cgroup, guest image, and guest-agent configuration it uses.

## Current architecture vs. future distribution

The core execution architecture is designed so that the control plane and privileged VM execution responsibilities can be separated further as Shiyao scales across hosts.

The current repository should be treated as the source of truth for which control-plane services, queues, persistence layers, and worker processes are actually implemented. Distributed components such as an external event bus or horizontally scaled worker fleet should not be assumed merely from this conceptual separation.

## Design principles

1. **Treat agent code as untrusted.** Isolation starts at the VM boundary.
2. **Keep execution bounded.** Requests, concurrency, output, lifetime, and resources should have explicit limits.
3. **Make network access explicit.** Default-deny is preferable to trying to enumerate everything a workload must not reach.
4. **Make resource ownership explicit.** Every VM should have a clear owner for its IP, TAP device, VSOCK CID, socket, and Firecracker process.
5. **Prefer disposable state.** Workload state should not become durable merely because a VM is reused.
6. **Optimize without weakening isolation.** Warm pools and snapshots exist to reduce startup cost while preserving the VM boundary.
7. **Design for failure.** Provisioning and teardown can fail independently; cleanup and reconciliation are part of the architecture.
