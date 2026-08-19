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

## 🔧 Configuration: `shiyao.yaml`

Shiyao uses a declarative YAML file to define **snapshots** – pre‑built, reusable microVM images. This file controls everything from base OS and dependencies to resource limits and network policies.

### Schema Reference

| Field                       | Type                | Required | Default                                           | Description                                                                                                 |
| --------------------------- | ------------------- | -------- | ------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `version`                   | `string`            | No       | `"v1alpha1"`                                      | Schema version (for future compatibility).                                                                  |
| `name`                      | `string`            | **Yes**  | —                                                 | Human‑readable name for the snapshot (used as identifier).                                                  |
| `description`               | `string`            | No       | `""`                                              | Optional description of the environment.                                                                    |
| `runtime`                   | `object`            | No       | `{os: linux, distro: ubuntu-22.04, arch: x86_64}` | Base OS configuration.                                                                                      |
| `runtime.os`                | `string`            | No       | `"linux"`                                         | Only `linux` is supported.                                                                                  |
| `runtime.distro`            | `string`            | No       | `"ubuntu-22.04"`                                  | Distribution name (e.g., `ubuntu-22.04`, `debian-12`).                                                      |
| `runtime.architecture`      | `string`            | No       | `"x86_64"`                                        | CPU architecture (`x86_64` only for now).                                                                   |
| `language`                  | `object`            | No       | —                                                 | Primary language runtime (e.g., Python, Node).                                                              |
| `language.name`             | `string`            | No       | `"python"`                                        | Language name (`python`, `node`, `go`, etc.).                                                               |
| `language.version`          | `string`            | No       | `"3.11"`                                          | Version string (e.g., `3.11`, `18.17`).                                                                     |
| `dependencies`              | `object`            | No       | —                                                 | Lists of packages to install.                                                                               |
| `dependencies.system`       | `array` of `string` | No       | `[]`                                              | System packages (installed via `apt-get`).                                                                  |
| `dependencies.pip`          | `array` of `string` | No       | `[]`                                              | Python packages (installed via `pip`).                                                                      |
| `dependencies.npm`          | `array` of `string` | No       | `[]`                                              | Node.js global packages (installed via `npm -g`).                                                           |
| `resources`                 | `object`            | **Yes**  | —                                                 | CPU, memory, and disk limits.                                                                               |
| `resources.vcpu`            | `integer`           | **Yes**  | —                                                 | Number of vCPUs (≥ 1).                                                                                      |
| `resources.memory_mb`       | `integer`           | **Yes**  | —                                                 | Memory in MiB (≥ 128, recommended ≥ 512).                                                                   |
| `resources.disk_mb`         | `integer`           | **Yes**  | —                                                 | Disk size in MiB (≥ 1024).                                                                                  |
| `env`                       | `object`            | No       | `{}`                                              | Environment variables (key‑value) set inside the VM.                                                        |
| `network`                   | `object`            | No       | `{block_private_ips: true}`                       | Network egress policies.                                                                                    |
| `network.allowed_domains`   | `array` of `string` | No       | `[]`                                              | Domains the VM may connect to (e.g., `["api.openai.com"]`). If empty, all egress is blocked (default‑deny). |
| `network.block_private_ips` | `boolean`           | No       | `true`                                            | Whether to block private IP ranges (prevents SSRF).                                                         |

> **Note:** `system` dependencies are installed using `apt-get update -y` and `apt-get install -y`. Ensure your base image has a working package manager.

---

### ✅ Validation Rules

- `name` – must be non‑empty.
- `vcpu` – must be > 0.
- `memory_mb` – must be ≥ 128 (minimum for Firecracker).
- `disk_mb` – must be ≥ 1024 (1 GiB) to fit a basic OS.

If any of these fail, the build will abort with a clear error message.

---

### 📝 Example `shiyao.yaml`

```yaml
version: "v1alpha1"
name: "ml-inference-agent"
description: "PyTorch + Transformers for code‑generation agents"

runtime:
    os: linux
    distro: ubuntu-22.04
    architecture: x86_64

language:
    name: python
    version: "3.11"

dependencies:
    system:
        - curl
        - git
        - build-essential
    pip:
        - torch==2.0.1
        - transformers>=4.30.0
        - accelerate
        - playwright==1.40.0
    npm:
        - puppeteer@21.0.0

resources:
    vcpu: 2
    memory_mb: 4096
    disk_mb: 5120

env:
    PYTHONUNBUFFERED: "1"
    TRANSFORMERS_CACHE: "/tmp/cache"

network:
    allowed_domains:
        - api.openai.com
        - huggingface.co
    block_private_ips: true
```

---

### 🧠 How It Works

1. **Build**: When you run `shiyao snapshot build --config shiyao.yaml`, the builder:
    - Copies the base OS image.
    - Mounts it via loopback.
    - `chroot`s into it and runs all `apt`, `pip`, and `npm` commands.
    - Sets environment variables.
    - Shrinks the filesystem to save space.
    - Saves the resulting image as a snapshot.

2. **Run**: Later, when you execute code with `shiyao run --snapshot <name>`, the daemon boots a Firecracker microVM from that snapshot in ~125ms.

---

### 🛠️ Tips

- **Pin package versions** (e.g., `torch==2.0.1`) for reproducible builds.
- Keep `disk_mb` as low as possible – the builder will automatically shrink the filesystem after installation, but you still pay for storage.
- Use `allowed_domains` to enforce strict egress – the default‑deny policy adds a strong security layer.
- For heavy dependencies (PyTorch, TensorFlow), consider building a dedicated snapshot once and reusing it across all your agents.

---

### 🚧 Planned Enhancements

- Support for `pre_install` and `post_install` scripts.
- Full‑memory snapshots (restore in < 50ms).
- Integration with remote snapshot registries (S3, OCI).
