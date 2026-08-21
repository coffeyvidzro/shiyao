# Architecture

Shiyao is designed with a strict separation between the control plane (API and state management) and the data plane (hypervisor and workload execution). This allows the API to scale horizontally while the hypervisor workers remain pinned to bare-metal hosts with KVM access.

## Core Components

### 1. API Daemon (`cmd/daemon`)
The public-facing control plane. It handles HTTP/WebSocket requests, authenticates users, manages state in PostgreSQL, and dispatches asynchronous tasks.
- **Framework**: Gin (HTTP) and Gorilla WebSocket.
- **State**: PostgreSQL via `sqlc` for strict typing and query safety.
- **Event Bus**: NATS JetStream publisher for asynchronous VM lifecycle events.

### 2. Worker (`cmd/worker`)
The privileged data plane. It runs on bare-metal Linux hosts and listens for NATS events to provision, manage, and destroy Firecracker microVMs.
- **VMM Manager**: Orchestrates Firecracker instances, handling admission control (backpressure) and warm instance pooling.
- **Network/IPAM**: Allocates TAP devices, manages guest IP subnets, and applies per-VM `nftables` policies.
- **Firecracker SDK**: Interfaces with the Firecracker API socket to configure and boot microVMs.

### 3. Guest Agent (`cmd/guest-agent`)
A minimal, statically compiled Go binary that runs as `init` (PID 1) inside the Firecracker microVM.
- **Execution Server**: Listens on a VSOCK port for execution requests from the host.
- **Resource Enforcement**: Applies `cgroup v2` limits (CPU, memory, PIDs) and `rlimits` to executed commands.
- **Ephemeral Rootfs**: Uses `OverlayFS` and `PivotRoot` to ensure the guest filesystem is strictly ephemeral.

## Data Flow: Sandbox Execution

1. **Request**: A developer sends a `POST /v1/sandboxes` request to the API Daemon.
2. **State**: The Daemon validates the request, creates a `pending` record in PostgreSQL, and publishes a `sandbox.create` event to NATS.
3. **Provisioning**: The Worker consumes the NATS event, acquires an IPAM lease, creates a TAP device, and applies `nftables` egress rules.
4. **Boot**: The Worker boots the Firecracker microVM (either cold boot or snapshot resume) and transitions the database state to `running`.
5. **Execution**: The developer connects via WebSocket (`/v1/sandboxes/:id/exec/stream`). The Daemon bridges this WebSocket to the Guest Agent via VSOCK.
6. **Teardown**: Upon timeout or explicit deletion, the Worker stops the VMM, releases the IPAM lease, removes the TAP device, and updates the database state to `stopped`.

## Network Topology

```text
[ Internet ]
      |
[ Host nftables (Forward Chain) ] <--- Default DROP, custom SHIYAO chains per VM
      |
[ Host TAP Device (shiyao_tap_*) ]
      | (VirtIO Net)
[ Firecracker microVM ]
      |
[ Guest Agent (VSOCK Port 1024) ]
