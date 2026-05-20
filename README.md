# Nomici Orchestrator

Open-source control plane and designer for local and remote AI agents.

Nomici Orchestrator is a local-first agent control plane for configuring LLM providers, installing useful agent packs, running and observing agent runtimes, mediating tools, and governing traces, approvals, artifacts, and policies.

> Project status: v0.1 alpha bootstrap. Architecture and APIs are still being refined. The minimal CLI/Gateway scaffold, default Gateway token auth, dynamic provider/model catalog, guided provider setup, Gateway-mediated model test, OpenAI-compatible `/v1/models` and `/v1/chat/completions`, bundled `developer-team` pack install with role metadata, minimal AgentSpec/AgentGraph validation, single-node model-backed graph execution, generic `cli_agent` external-agent execution, one-step `cli_agent` handoff with Shared Context snapshots, durable chat/session/task APIs, session inspection CLI commands, sequential role task execution with plan review, local workspace uploads, typed plan/report artifacts, mutable `cli_agent` approval gates, minimal `local_process` lifecycle commands, trace inspection commands, and chat-first Console workspace panels are implemented. General parallel multi-agent graph execution, broader policy coverage, and Console setup editing workflows are not implemented yet.

## Why Nomici

Modern agents are becoming powerful but fragmented. A developer may have Claude Code, Codex, opencode, Aider, Cline, Continue, a Hermes coder, an OpenClaw operator, an Ollama model, a LangGraph workflow, several MCP tools, office documents, and multiple API keys spread across terminals, configs, ports, and dashboards.

Nomici aims to provide one local-first control plane for composing those pieces into useful, inspectable, governed agent organizations.

## Quickstart

```bash
git clone https://github.com/NomiciAI/nomici-orchestrator.git
cd nomici-orchestrator
scripts/install.sh --from-source .
nomici setup
nomici dev
nomici run product_pm "Plan the first useful task."
```

The source installer checks the required build tools and activates `pnpm` through Corepack when needed. The hosted `curl` install endpoint and release artifacts are still planned.

See [Quickstart](docs/quickstart.md) for the guided setup path, scriptable setup flags, and the optional no-API-key smoke test.

## Implemented Bootstrap Commands

The current alpha can be set up and started from the repository root:

```bash
scripts/install.sh --from-source .
nomici setup
nomici dev
nomici run product_pm "Verify local workspace execution."
```

Lower-level and inspection commands remain available for scripts and advanced workflows:

```bash
nomici model setup --kind openai_compatible --name gpt --model <model> --api-key-env OPENAI_API_KEY
nomici model list
nomici model doctor
nomici provider list
nomici provider models openai --search gpt
nomici provider doctor openai
nomici pack list
nomici pack inspect developer-team
nomici pack install developer-team --model gpt
nomici spec validate --config nomici.yaml
nomici graph export --config nomici.yaml --format json
nomici runtime inspect implementer_cli --config nomici.yaml
nomici up
nomici dev --no-open
nomici gateway status
nomici ps
nomici logs gateway --tail 50
nomici gateway token show
nomici gateway open
curl -H "Authorization: Bearer $(nomici gateway token show)" http://127.0.0.1:8787/v1/models
nomici model test gpt "Say hello from Nomici"
nomici run model gpt "Say hello from Nomici"
nomici run product_pm "Say hello through a single-node graph"
nomici agent run implementer "Use a configured cli_agent runtime"
nomici run implementer "Run a supported cli_agent handoff chain"
nomici context list
nomici policy check
nomici approvals list
nomici approvals grant <approval_id> --scope once
nomici approvals deny <approval_id>
nomici session list
nomici session show <session_id>
nomici session tasks <session_id>
nomici session cancel <session_id>
nomici upload add <path> --session <session_id>
nomici upload list --session <session_id>
nomici artifact list --session <session_id>
nomici artifact show <artifact_id>
nomici trace list
nomici trace show <run_id>
nomici down
```

`nomici setup` is the recommended first-run path. It keeps the existing `nomici model setup`, `nomici pack install`, `nomici up`, and `nomici doctor` commands as scriptable building blocks, but wraps the common path in one guided flow:

```text
LLM provider -> live model catalog -> web search/fetch -> starter pack -> sandbox policy -> nomici.yaml -> nomici dev
```

The setup command writes model, web search/fetch, starter pack, and sandbox intent into the local workspace. Provider selection is two-level: choose a provider, then pick from that provider's live model catalog where the provider exposes one. Local CLI providers and custom OpenAI-compatible endpoints allow an explicit model id when the model list cannot be discovered. v0.1 treats the web tool entries as read-only provider contracts until mediated tool execution is enabled. `nomici doctor` reports whether sandbox config exists, whether requested web providers have required env vars, and whether a requested container sandbox has a local container runtime such as Docker or Podman.

Provider setup stores only secret references such as `OPENAI_API_KEY`; raw API keys are not stored in `nomici.yaml` or the local SQLite profile store.

The bundled `developer-team` pack installs a runnable `product_pm` coordinator entrypoint plus planner, researcher, coder, and reporter role agents using an existing model provider profile. Its manifest also exposes role purpose, tool and skill expectations, handoff mode, model/runtime preference, and output contract metadata so run task ledgers can show role ownership without hardcoding those roles in Gateway. It also saves a graph snapshot for Console. Optional CLI-backed implementer/reviewer roles are represented in the pack design but are not installed by default yet.

`nomici dev` starts the local Gateway, validates the configured graph, starts configured local processes, and opens Console. The current Console is served by Gateway and defaults to Chat. Each chat can start a workspace run, while Orchestrate and Settings expose task ledger records, workspace roots, uploads, artifacts, plan review actions, trace events, approvals, provider/model catalog status, configured model profiles, and tool contracts. It does not read local files or receive raw secret values. API calls require the local Gateway token; run `nomici gateway token show` from the same project directory that started Gateway and paste it into the Console when prompted.

The current graph runner supports a single executable `gateway_agent` or `model_agent` node backed by an implemented model profile, a single `external_agent` backed by a configured `cli_agent` runtime, or a linear `handoff` chain across `cli_agent`-backed `external_agent` nodes. Branching handoffs, parallel, A2A, and tool edges validate structurally but fail clearly if executed.

The intended v0.1 flow:

- Check the local machine with `nomici doctor`.
- Configure an LLM provider without writing raw secrets to `nomici.yaml`.
- Install a useful agent pack.
- Review requested permissions.
- Start Nomici Gateway on `http://127.0.0.1:8787`.
- Start configured local runtimes.
- Run a real task.
- Inspect traces, logs, approvals, artifacts, and policy decisions.

The optional `examples/basic-local-agent` directory remains useful as a no-provider smoke test for local `cli_agent` execution, but it is not required for normal setup. In the current alpha bootstrap, `nomici doctor` performs local checks and `nomici dev` opens Console. The source installer works from a local checkout; hosted release downloads remain planned.

`NOMICI_GATEWAY_URL` is a local CLI convenience for pointing commands at a running Gateway. CLI commands authenticate with `NOMICI_GATEWAY_TOKEN` when set, otherwise they read `.nomici/gateway.token` next to the local state database. Treat the token as an operator credential and do not point the CLI at a shared or public Gateway unless that Gateway is explicitly trusted.

## What Nomici Will Do

- Guided LLM provider setup
- First-run useful agent packs
- Shared context and handoff briefings across agents
- Visual agent canvas
- Local runtime manager
- AgentSpec YAML
- Model registry
- Runtime registry
- MCP tool and data integration
- A2A agent-to-agent integration
- OpenAI-compatible model and agent endpoint
- Generic CLI agent runner for command-driven agents such as Claude Code, Codex, opencode, Aider, and custom commands
- Tool packs for local work such as office documents, developer workflows, research, and personal ops
- Trace, replay, approval, policy, and audit logs

## What Nomici Is Not

Nomici is not:

- A model training system.
- A replacement for Hermes, OpenClaw, LangGraph, CrewAI, or OpenAI Agents SDK.
- A cloud-first hosted platform.
- A general-purpose workflow engine in v0.1.
- A security boundary for arbitrary untrusted local code.

Nomici should manage, connect, and govern strong runtimes rather than rebuild them.

Nomici may include a minimal `gateway_agent` loop for first-run coordination. Durable or framework-native agent behavior belongs in external runtimes.

## Architecture Direction

Nomici uses a Gateway-centered architecture. The current authoritative summary is [Architecture](docs/architecture.md).

```text
Nomici Console
  -> Nomici Gateway
    -> Agent registry
    -> Runtime manager
    -> Model registry
    -> Policy and approvals
    -> Trace store
    -> Adapter layer
      -> CLI agents / Hermes / OpenClaw / local models / MCP / A2A
```

The current stack is:

- Go for CLI, Gateway, runtime manager, policy, trace, and release binary.
- TypeScript, React, Vite, React Flow, plain CSS, and pnpm for Nomici Console.
- SQLite by default.
- JSON Schema for AgentSpec.
- A single `nomici` binary for end users.
- `make` commands as the contributor interface.

The current development plan is [Development Plan](docs/development-plan.md).

The current living implementation checklist is [v0.1 Implementation Plan](docs/implementation/v0.1-plan.md).

Implementation design notes live in [Design Deep Dives](docs/design/README.md).

## RFCs

Current design documents:

- [RFC Index](docs/rfcs/README.md)
- [RFC 0001: Product Scope](docs/rfcs/0001-product-scope.md)
- [RFC 0002: Architecture](docs/rfcs/0002-architecture.md)
- [RFC 0003: AgentSpec v0.1](docs/rfcs/0003-agentspec-v0.1.md)
- [RFC 0004: Security Model](docs/rfcs/0004-security-model.md)
- [RFC 0005: Technology Stack Decision](docs/rfcs/0005-technology-stack.md)
- [RFC 0006: Control Plane Architecture](docs/rfcs/0006-control-plane-architecture.md)
- [RFC 0007: AgentGraph IR and Adapter Contract](docs/rfcs/0007-agentgraph-ir-adapter-contract.md)
- [RFC 0008: First-Run Experience, Provider Setup, and Packs](docs/rfcs/0008-first-run-provider-setup-and-packs.md)
- [RFC 0009: Application and Engineering Architecture](docs/rfcs/0009-application-and-engineering-architecture.md)

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
