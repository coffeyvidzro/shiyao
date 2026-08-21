# Security Model

Shiyao is designed to execute untrusted, AI-generated code. The security model relies on defense-in-depth, combining hardware virtualization, strict network policies, and rigorous resource limits.

## 1. Hardware Isolation (Firecracker)
Every sandbox runs inside a dedicated Firecracker microVM. 
- **Minimal Attack Surface**: Firecracker strips away unnecessary legacy devices, exposing only the bare minimum required to boot a Linux kernel.
- **KVM Isolation**: Workloads are isolated by the Linux kernel's KVM hardware virtualization, preventing guest-to-host and guest-to-guest escape.

## 2. Network Policy (The "Stone" Boundary)
Guest networking is strictly default-deny. The host uses `nftables` to enforce egress policies per TAP device.
- **Metadata Protection**: Access to cloud metadata endpoints (e.g., `169.254.169.254`) is explicitly blocked at the network layer to prevent credential leakage.
- **SSRF Prevention**: The host-side proxy transport resolves DNS immediately before dialing and validates the IP against a strict allowlist of public, global unicast addresses. This prevents DNS rebinding attacks.
- **Egress Allowlisting**: Traffic is only permitted to explicitly configured domains/ports (e.g., UDP 53 for DNS, TCP 443 for HTTPS).

## 3. Resource Limits & Containment
To prevent resource exhaustion (fork bombs, memory leaks) from impacting the host or other tenants:
- **cgroup v2**: Every executed command inside the guest is placed in an isolated cgroup with strict limits on CPU quota, memory (e.g., 384 MiB max), and PIDs (e.g., 256 max).
- **rlimits**: Process-level limits are applied via `prlimit` to cap maximum file sizes and open file descriptors.
- **Output Bounding**: Stdout and stderr capture are strictly bounded (e.g., 10 MiB per stream). If a command exceeds this, the output is truncated to prevent host memory exhaustion.

## 4. Ephemeral Filesystem
The guest agent replaces the root filesystem at boot using `OverlayFS` and `PivotRoot`. 
- The lower layer is a read-only, immutable base image.
- The upper layer is a `tmpfs` in memory.
- When the microVM is destroyed, the `tmpfs` is wiped, guaranteeing zero state persistence between tenants.

## 5. Secure Host-Guest Communication
Communication between the Shiyao control plane and the guest agent occurs exclusively over **VSOCK** (VirtIO VSOCK), not the guest's network interface.
- **CID Authorization**: The guest agent verifies the Context ID (CID) of incoming VSOCK connections, ensuring only the host hypervisor can issue execution commands.
- **Protocol Validation**: The VSOCK protocol enforces strict limits on request sizes (1 MiB), environment variable keys/values, and concurrent executions (max 4 per guest).

## 6. Data Plane Privileges
- The **API Daemon** runs as an unprivileged user and never has access to KVM or raw network configuration.
- The **Worker** runs with elevated privileges (root/CAP_NET_ADMIN) but is never exposed to the public internet. It only consumes internal NATS events.