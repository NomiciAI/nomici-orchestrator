# RFC 0001: Product Scope

Status: Draft
Date: 2026-04-30
Target release: Nomici Orchestrator v0.1

## Summary

Nomici Orchestrator is a local-first, open-source control plane for local and remote AI agents.

The first release should prove one narrow promise:

> A user can describe an agent organization in `nomici.yaml`, start it with `nomici up`, inspect it in Nomici Console, run an agent, and see traces, logs, policy decisions, and approvals through Nomici Gateway.

Nomici should not be positioned as another multi-agent framework. It should be positioned as the open control plane for agent organizations: a layer above strong agent runtimes, local model servers, MCP servers, and A2A-compatible remote agents.

## Product Positioning

Brand: Nomici Orchestrator
Org: nomici.ai
GitHub repo: NomiciAI/nomici-orchestrator
CLI: `nomici`
Config file: `nomici.yaml`
Gateway: Nomici Gateway
Web UI: Nomici Console

Tagline:

> Open-source control plane for local and remote AI agents.

Primary category:

> Agent organization control plane.

Description:

> Nomici Orchestrator lets users run, register, connect, observe, and govern multiple AI agent runtimes from one local-first Gateway, CLI, Web Console, and versioned AgentSpec.

## Users

The target users for v0.1 are technical users who already use or evaluate agent systems:

- AI application builders
- Coding agent and automation power users
- Local model users
- Small-team AI infrastructure builders
- Open-source agent framework developers
- Internal enterprise agent platform teams

The user problem:

> I already have tools like Claude Code, Codex, opencode, Aider, Cline, Continue, Hermes, OpenClaw, Ollama, vLLM, LangGraph, OpenAI Agents SDK, CrewAI, and MCP servers, but they are scattered across terminals, ports, configs, and dashboards. I need one local-first control plane to register them, connect them, govern them, and observe runs.

## Goals

v0.1 must support these product goals:

- Define an agent organization in `nomici.yaml`.
- Start Nomici Gateway and local runtimes with `nomici up`.
- Register models, agents, runtimes, MCP tools, and graph edges.
- Run native or external agents through a common interface.
- Carry shared context across handoffs without replacing agent-native memory.
- Manage local process runtimes with PID, port, health, and logs.
- Connect to OpenAI-compatible endpoints, including local model servers and external agent gateways.
- Provide a Web Console with dashboard, canvas, runtimes, models, agents, runs, traces, approvals, and settings.
- Persist event logs and operational state in SQLite.
- Enforce conservative default policy and approval rules.
- Provide CLI commands for all core Web Console operations.
- Ship demo templates that work on a local machine.

## Non-Goals

v0.1 must not attempt to do these things:

- Train models.
- Rebuild Hermes, OpenClaw, LangGraph, CrewAI, or OpenAI Agents SDK.
- Become a full cloud platform first.
- Support every agent framework.
- Provide full multi-user RBAC, OIDC, or enterprise tenancy.
- Provide a marketplace or remote package registry.
- Promise deterministic re-execution of arbitrary LLM or external-agent runs.
- Provide deep native adapters for every framework.
- Expose the Gateway to public networks by default.

## v0.1 Scope

v0.1 is named:

> Nomici Orchestrator v0.1: Local Agent Control Plane

Included:

- `nomici` CLI
- Nomici Gateway
- Basic Nomici Console
- AgentSpec v0.1
- JSON Schema for AgentSpec
- Model registry
- Agent registry
- Runtime registry
- Local process runner
- OpenAI-compatible endpoint adapter
- Generic CLI Agent Runner
- CLI agent profiles for Codex, Claude Code, opencode, Aider, custom commands, and editor-native agents where they expose automation surfaces
- Hermes endpoint adapter
- OpenClaw endpoint adapter
- Ollama and vLLM through OpenAI-compatible or provider-specific configuration
- SQLite trace store with WAL
- Shared Context Layer for project/run context and handoff snapshots
- Approval queue
- Policy defaults for high-risk tools
- 3 demo templates:
  - AI Application PM
  - PR Review Agents
  - Local Research Swarm
- Local install script target, even if initially backed by development builds

Deferred:

- A2A sidecars
- Native Hermes profile manager beyond process and endpoint management
- Native OpenClaw gateway protocol integration
- LangGraph adapter
- OpenAI Agents SDK trace bridge
- CrewAI, ADK, and Agent Squad adapters
- Postgres
- Kubernetes
- Helm chart
- Multi-user server deployment
- Template marketplace
- Signed adapter registry

## Required User Journey

The first successful user journey should be:

```bash
curl -fsSL https://nomici.ai/install.sh | bash
nomici init --template ai-application-pm
nomici up
nomici gateway open
nomici run product_pm "Design an AI app for local coding agents"
nomici trace list
nomici trace show <run_id>
```

Expected result:

- Gateway starts on `http://127.0.0.1:8787`.
- Console opens from the Gateway.
- Config is loaded from `nomici.yaml`.
- Local runtimes are started if configured.
- Agent graph is visible in the Console.
- A run can be started from CLI or Web UI.
- Events are persisted in SQLite.
- Risky actions create approvals before execution.

## Product Principles

Local-first:

Nomici must work well on a single developer machine before it supports teams or cloud deployment.

Control plane, not replacement runtime:

Nomici should manage and govern agent runtimes rather than replace them.

Spec-first:

Agent organizations should be versionable, reviewable, and reproducible from `nomici.yaml`.

Gateway-centered:

CLI, Web Console, API clients, and OpenAI-compatible clients should converge through Nomici Gateway.

Security by default:

Loopback bind, token auth, secret redaction, untrusted remote agents, and approval-first risky actions must be defaults, not optional hardening.

Adapter pragmatism:

OpenAI-compatible endpoints and generic CLI agent invocation are the v0.1 integration paths. Deeper native adapters come only after the control plane is stable.

Observable by default:

Every run should produce structured events, logs, policy decisions, approvals, and exportable traces.

## v0.1 Success Criteria

Nomici v0.1 is successful when:

- A fresh user can run a demo template in under 10 minutes on macOS or Linux.
- The Gateway and Console start from one binary.
- `nomici.yaml` validates with useful errors.
- At least one external OpenAI-compatible agent endpoint can be registered and invoked.
- Hermes and OpenClaw can be represented as managed external runtimes.
- Local model endpoints can be registered as model nodes.
- A run produces a timeline in the Console and JSONL export from the CLI.
- Shell and filesystem-write-like actions require approval by default when mediated through Nomici.
- Debug bundles redact secrets.
- The project has enough docs for a new contributor to implement an adapter.

## Open Questions

- Should `nomici up` always start Gateway, or should it fail if Gateway is already running with a different workspace?
- Should v0.1 allow multiple active workspaces per profile, or one active workspace at a time?
- Settled after RFC 0009 and later boundary review: `gateway_agent` is the name for a minimal Gateway-run coordinator loop, but the first compelling developer proof may use external `cli_agent` runtimes instead. Richer agent loops remain external runtimes.
- `gateway_agent` is a single-run coordinator, not a durable or self-directed multi-step agent runtime.
- Should Docker runner be included in v0.1, or deferred to v0.2?
- Should the OpenAI-compatible `/v1/*` endpoint be enabled by default or require explicit opt-in?
