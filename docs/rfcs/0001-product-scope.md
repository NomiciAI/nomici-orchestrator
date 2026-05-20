# RFC 0001: Product Scope

Status: Draft
Date: 2026-04-30
Target release: Nomici Orchestrator v0.1

## Summary

Nomici Orchestrator is a local-first, open-source long-horizon agent harness for orchestrating, observing, and governing multi-agent AI work.

The first release should prove one narrow promise:

> A user can run `nomici setup`, start `nomici dev`, describe a goal in Chat, and watch Nomici route the request into either a normal answer or a governed harness run with agents, tools, artifacts, review, and resumable history.

Nomici should not be positioned as another multi-agent framework. It should be positioned as an agent harness: a local product and engineering surface above strong model providers, local runtimes, MCP servers, tools, packs, and future workflow adapters.

## Product Positioning

Brand: Nomici Orchestrator
Org: nomici.ai
GitHub repo: NomiciAI/nomici-orchestrator
CLI: `nomici`
Config file: `nomici.yaml`
Gateway: Nomici Gateway
Web UI: Nomici Console

Tagline:

> Open-source long-horizon agent harness for orchestrating, observing, and governing multi-agent AI work.

Primary category:

> Long-horizon agent orchestration harness.

Description:

> Nomici Orchestrator lets users explore how far multi-agent AI work can go today by turning chat goals into governed harness runs with explicit routing, agent selection, tool execution, review checkpoints, artifacts, memory proposals, and durable timelines.

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

- Configure the first usable model, tools, starter pack, and sandbox policy with `nomici setup`.
- Start the complete local product with `nomici dev`.
- Make Chat the default entrypoint for both direct answers and long-horizon runs.
- Route user intent into direct reply, clarification, or workspace run without requiring users to know agent IDs.
- Provide Agent Studio for creating, testing, validating, and saving agents without hand-editing YAML.
- Provide Orchestration Studio for sequential role flow editing, preview, testing, and review policy.
- Register models, agents, runtimes, MCP tools, and graph edges.
- Run model, gateway, and external agents through a common harness interface.
- Carry shared context across handoffs without replacing agent-native memory.
- Manage local process runtimes with PID, port, health, and logs.
- Connect to OpenAI-compatible endpoints, including local model servers and external agent gateways.
- Provide a Web Console with Chat, Agents, Orchestration, Runs, Settings, timeline, review queue, artifacts, tools, and memory.
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

> Nomici Orchestrator v0.1: Local Long-Horizon Agent Harness

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
- Chat-first router, agent matcher, and suggestions
- Agent Studio and Orchestration Studio
- Harness run timeline, todos, tool calls, artifacts, review queue, and memory proposals
- Local eval harness for router and multi-agent workflow probes
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
scripts/install.sh --from-source .
nomici setup
nomici dev
```

Expected result:

- Gateway starts on `http://127.0.0.1:8787`.
- Console opens from the Gateway.
- Chat is immediately usable for direct questions.
- Complex goals are automatically routed into a harness run.
- Agent and role selection is explained before and during execution.
- Plans, review requests, tool calls, artifacts, todos, and timeline are visible in context.
- Agent creation and orchestration editing are available from Console.
- Config is loaded from shared project config and local overrides.
- Local runtimes and sandbox providers are started or diagnosed if configured.
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
