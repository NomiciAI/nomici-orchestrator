# Nomici Orchestrator

Open-source control plane for local and remote AI agents.

Nomici Orchestrator is a local-first agent control plane for running, registering, connecting, observing, and governing agent runtimes such as Hermes, OpenClaw, local model servers, MCP tools, and A2A-compatible remote agents.

> Project status: design and RFC phase. The commands below describe the intended v0.1 user experience and are not implemented yet.

## Why Nomici

Modern agents are becoming powerful but fragmented. A developer may have a Hermes coder, an OpenClaw operator, an Ollama model, a LangGraph workflow, several MCP tools, and multiple API keys spread across terminals, configs, ports, and dashboards.

Nomici aims to provide one local-first control plane for composing those pieces into manageable agent organizations.

## Intended Quickstart

```bash
curl -fsSL https://nomici.ai/install.sh | bash
nomici init --template ai-application-pm
nomici up
nomici gateway open
```

The intended v0.1 flow:

- Define an organization in `nomici.yaml`.
- Start Nomici Gateway on `http://127.0.0.1:8787`.
- Start configured local runtimes.
- Open Nomici Console.
- Run an agent.
- Inspect traces, logs, approvals, and policy decisions.

## What Nomici Will Do

- Visual agent canvas
- Local runtime manager
- AgentSpec YAML
- Model registry
- Runtime registry
- MCP tool and data integration
- A2A agent-to-agent integration
- OpenAI-compatible model and agent endpoint
- Hermes and OpenClaw adapters
- Trace, replay, approval, policy, and audit logs

## What Nomici Is Not

Nomici is not:

- A model training system.
- A replacement for Hermes, OpenClaw, LangGraph, CrewAI, or OpenAI Agents SDK.
- A cloud-first hosted platform.
- A general-purpose workflow engine in v0.1.
- A security boundary for arbitrary untrusted local code.

Nomici should manage, connect, and govern strong runtimes rather than rebuild them.

## Architecture Direction

Nomici uses a Gateway-centered architecture:

```text
Nomici Console
  -> Nomici Gateway
    -> Agent registry
    -> Runtime manager
    -> Model registry
    -> Policy and approvals
    -> Trace store
    -> Adapter layer
      -> Hermes / OpenClaw / local models / MCP / A2A
```

The planned stack is:

- Go for CLI, Gateway, runtime manager, policy, trace, and release binary.
- TypeScript, React, Vite, React Flow, Tailwind, and pnpm for Nomici Console.
- SQLite by default.
- JSON Schema for AgentSpec.
- A single `nomici` binary for end users.
- `make` commands as the contributor interface.

## RFCs

Current design documents:

- [RFC 0001: Product Scope](docs/rfcs/0001-product-scope.md)
- [RFC 0002: Architecture](docs/rfcs/0002-architecture.md)
- [RFC 0003: AgentSpec v0.1](docs/rfcs/0003-agentspec-v0.1.md)
- [RFC 0004: Security Model](docs/rfcs/0004-security-model.md)
- [RFC 0005: Technology Stack Decision](docs/rfcs/0005-technology-stack.md)

## Security Defaults

Nomici is a control plane and must be conservative by default:

- Gateway binds to `127.0.0.1` by default.
- Gateway token auth is enabled by default.
- Remote access is disabled by default.
- Secrets are referenced, not stored in `nomici.yaml`.
- MCP servers and remote agents are untrusted by default.
- Shell, filesystem write, email, deploy, and unknown network actions require approval by default.
- OpenAI-compatible `/v1/*` endpoints are treated as operator-level surfaces unless scoped differently in a future release.

See [SECURITY.md](SECURITY.md) and [RFC 0004](docs/rfcs/0004-security-model.md).

Related guardrails:

- [Threat model](docs/security/threat-model.md)
- [Dangerous changes checklist](docs/security/dangerous-changes.md)
- [Supply chain security](docs/security/supply-chain.md)
- [Privacy](docs/privacy.md)
- [Open source publication checklist](docs/release/open-source-checklist.md)

## Contributing

Nomici is early. Contributions should start with small, clear changes or RFC discussion.

Planned contributor commands:

```bash
make dev
make build
make test
make lint
make fmt
```

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Nomici Orchestrator is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).
