# Shiyao Roadmap

This document outlines the development roadmap for Shiyao. Items are grouped by priority and timeline.

## Completed (Phase 1: Core Foundation)

### VMM Engine
- [x] Firecracker microVM lifecycle management (create, start, stop)
- [x] State machine for VM lifecycle transitions
- [x] Snapshot support with integrity verification
- [x] Warm instance pooling with safe reset operations
- [x] Admission control with backpressure handling
- [x] Cold boot vs snapshot resume benchmarking

### Networking & Security
- [x] Per-VM TAP device management
- [x] nftables-based network policy (default-deny forwarding)
- [x] IP address management (IPAM) for guest subnets
- [x] VSOCK CID allocation and validation
- [x] SSRF protection with DNS rebinding prevention
- [x] Cloud metadata endpoint blocking (169.254.169.254)

### Guest Agent
- [x] VSOCK-based command execution protocol
- [x] Bounded stdout/stderr capture (10 MiB limit)
- [x] Concurrent command limits (4 per guest)
- [x] Ephemeral rootfs via OverlayFS + PivotRoot
- [x] cgroup v2 resource limits (CPU, memory, PIDs)
- [x] Process-level rlimits (file size, open files)
- [x] Environment variable allowlisting

### API & Integration
- [x] Gin HTTP API for sandbox lifecycle
- [x] WebSocket streaming for real-time execution output
- [x] Session-based authentication
- [x] Personal access tokens (PAT) for SDK access
- [x] Database schema with sqlc integration
- [x] NATS JetStream event publishing

---

## In Progress (Phase 2: Production Readiness)

### Testing & CI/CD
- [ ] GitHub Actions workflow with KVM-enabled runners
- [ ] Integration tests for Firecracker boot/execute/destroy
- [ ] Load testing for concurrent VM provisioning
- [ ] Fuzz testing for VSOCK protocol parsing

### Deployment
- [ ] Docker Compose for local development (API + dependencies)
- [ ] Systemd service files for bare-metal deployment
- [ ] Automated migration deployment with Atlas
- [ ] Health check endpoints for API and worker

### Documentation
- [ ] API reference with OpenAPI/Swagger spec
- [ ] Architecture diagrams (data flow, network isolation)
- [ ] Deployment guide for Hetzner/Vultr bare metal
- [ ] Security model documentation (threat model, attack surface)

---

## Short-Term (Phase 3: Beta Release, Q1 2026)

### SDK & Developer Experience
- [ ] Python SDK with async support
- [ ] TypeScript/Node.js SDK
- [ ] CLI tool for local sandbox management
- [ ] Example integrations (LangChain, CrewAI, AutoGen)

### Monitoring & Observability
- [ ] Prometheus metrics for VM lifecycle events
- [ ] Structured logging with correlation IDs
- [ ] Resource usage tracking (CPU, memory, network egress)
- [ ] Audit logging for security compliance

### Performance Optimization
- [ ] Reduce cold boot time to <150ms
- [ ] Optimize snapshot resume to <50ms
- [ ] Connection pooling for VSOCK client
- [ ] Batch operations for bulk sandbox creation

---

## Medium-Term (Phase 4: Enterprise Features, Q2-Q3 2026)

### Multi-Tenancy & Teams
- [ ] Team-based access control
- [ ] Team-level API keys and quotas
- [ ] Resource isolation between tenants
- [ ] Usage-based billing integration (Polar)

### Advanced Networking
- [ ] Domain-level egress filtering (Layer 7)
- [ ] Custom proxy support for traffic inspection
- [ ] IPv6 support for guest networking
- [ ] Network policy templates for common use cases

### Snapshot Management
- [ ] Snapshot registry with versioning
- [ ] Automated snapshot creation from running VMs
- [ ] Snapshot sharing between users/teams
- [ ] Pre-warmed snapshot templates (Python, Node.js, Go)

---

## Long-Term (Phase 5: Scale & Ecosystem, Q4 2026+)

### Horizontal Scaling
- [ ] Multi-node worker cluster with load balancing
- [ ] Distributed IPAM for multi-host networking
- [ ] Cross-node VM migration (live or cold)
- [ ] Regional deployment support (US, EU, APAC)

### Compliance & Security
- [ ] SOC 2 Type II audit preparation
- [ ] ISO 27001 compliance documentation
- [ ] GDPR/PIPL data residency controls
- [ ] Hardware security module (HSM) integration for key management

### Ecosystem
- [ ] Plugin system for custom guest agents
- [ ] Marketplace for pre-built sandbox templates
- [ ] Webhook support for lifecycle events
- [ ] GraphQL API alternative to REST

---

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### High-Priority Areas for Contributors
- SDK development (Python, TypeScript, Go)
- Documentation and examples
- Integration tests
- Performance benchmarking
- Security audits

---

## Feedback

This roadmap is a living document. If you have feature requests or use cases that aren't covered, please open a GitHub issue or reach out to the maintainers.

**Last Updated:** August 2026