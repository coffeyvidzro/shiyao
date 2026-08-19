# Security Policy

Shiyao runs untrusted code in Firecracker microVMs, so security reports are taken seriously.

## Reporting a vulnerability

Please do not open a public issue for a suspected vulnerability. Email the maintainers at `security@shiyao.dev` with:

- a short description of the issue,
- reproduction steps or a proof of concept,
- affected versions or commits, and
- any known mitigations.

We will acknowledge reports within 5 business days and follow up with triage status after investigation.

## Scope

In scope:

- microVM escape or sandbox isolation bypass,
- host network policy bypass,
- unauthorized access to PostgreSQL, Redis, or NATS data,
- authentication or session handling flaws,
- unsafe defaults in deployment files.

Out of scope:

- denial-of-service issues that require local shell access,
- findings against development-only credentials in `deploy/compose.yaml`,
- vulnerabilities in unsupported forks or unmodified third-party dependencies.

## Deployment guidance

The files under `deploy/` are for local development. For production deployments:

- replace all development passwords,
- restrict PostgreSQL, Redis, and NATS to private networks,
- enable TLS for public endpoints,
- run Firecracker only on hosts with `/dev/kvm` access limited to trusted service users,
- keep guest kernels and root filesystems patched,
- review egress allowlists before exposing sandbox execution to users.
