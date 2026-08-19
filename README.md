# Shiyao (石爻)

**Secure, sub-second microVM execution environment for AI agents.**

Shiyao provides hardware-level isolated microVMs for AI agents to safely execute code, browse the web, and interact with tools. Powered by [Firecracker](https://firecracker-microvm.github.io/), it boots isolated environments in ~125ms, ensuring your AI agents can work at the speed of thought without compromising your infrastructure.

## Why Shiyao?

Modern AI agents (like AutoGen, CrewAI, or custom LLM workflows) need to execute code to accomplish tasks. But running untrusted, LLM-generated code on your servers is a massive security risk.

Shiyao solves this by spinning up ephemeral, strictly isolated microVMs for every execution.

- 🛡️ **Hardware-Level Isolation:** Built on Firecracker microVMs, providing stronger security boundaries than standard Docker containers.
- ⚡ **Sub-Second Boot Times:** Cold starts in ~125ms. No waiting for containers to pull or boot.
- 🌐 **Strict Network Control:** Default-deny egress networking. You define exactly which hosts the agent can talk to.
- 💾 **Instant Snapshots:** Pre-warm environments with heavy dependencies (PyTorch, Puppeteer) and load them instantly.

## The Name

**Shiyao (石爻)** combines two ancient concepts to represent our mission:

- **石 (Shí):** Stone / Bedrock. Represents the unbreakable, secure boundary of our infrastructure.
- **爻 (Yáo):** The binary lines of the _I Ching_. Represents the fundamental logic and computation of AI.

_We build the unbreakable stone vault where AI binary logic can safely execute._

## Architecture

Shiyao is written in **Go** for high-concurrency lifecycle management.

```text
[ AI Agent / LLM ]
       │
       ▼
[ Shiyao API (Go) ] ──> [ Firecracker MicroVM (Linux) ]
       │                       │
       ▼                       ▼
[ Control Plane ]       [ Isolated Code Execution ]
```

## Quick Start (Local Development)

_Note: Firecracker requires a Linux environment with KVM support. If you are on macOS or Windows, you will need to use a Linux VM or WSL2 with nested virtualization enabled._

### Prerequisites

- Go 1.26+
- Linux with KVM support (`/dev/kvm` must be accessible)

### Build and Run

```bash
# Clone the repository
git clone https://github.com/coffeyvidzro/shiyao.git
cd shiyao

# Install dependencies and build the daemon
make build

# Run the daemon locally (default port: 8080)
./bin/shiyao-daemon
```

### Test the API

Once the daemon is running, you can test the sandbox creation endpoint:

```bash
curl -X POST http://localhost:8080/v1/sandboxes \
  -H "Content-Type: application/json" \
  -d '{
    "template": "python-3.11",
    "timeout": 30
  }'
```

<!-- ## SDKs

We maintain officially supported SDKs in separate repositories to ensure clean release cycles and focused developer experiences:

*   🐍 **Python SDK:** [github.com/coffeyvidzro/shiyao-python](https://github.com/coffeyvidzro/shiyao-python)
*   🟦 **TypeScript SDK:** [github.com/coffeyvidzro/shiyao-js](https://github.com/coffeyvidzro/shiyao-js) -->

## Project Structure

```text
shiyao/
├── cmd/                  # Entry points (daemon, cli)
├── internal/             # Core logic (sandbox, api, security)
├── pkg/                  # Shared Go libraries and Firecracker wrappers
├── deploy/               # Dockerfile and local run scripts
├── test/                 # Integration and security tests
└── docs/                 # Architecture and API specifications
```

## Roadmap

- [x] Core Firecracker lifecycle management
- [x] Basic HTTP API for sandbox creation and code execution
- [ ] Strict egress network filtering (iptables/nftables integration)
- [ ] Filesystem snapshotting for pre-warmed environments
- [ ] WebSocket support for real-time stdout/stderr streaming

## Contributing

We welcome contributions! Please read our [Contributing Guidelines](CONTRIBUTING.md) before submitting a Pull Request.

_Note: Because Shiyao interacts with low-level hypervisor features, please ensure you have tested your changes in a Linux environment with KVM enabled._

## License

Shiyao is licensed under the **Apache 2.0 License**. See the [LICENSE](LICENSE) file for details.
