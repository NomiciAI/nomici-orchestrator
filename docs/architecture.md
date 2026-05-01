# Architecture

This document is the current authoritative architecture summary for Nomici Orchestrator.

RFCs remain the decision history. This document summarizes the current direction that implementation should follow.

Implementation-oriented details live in `docs/design/`.

## Product Architecture

Nomici Orchestrator is a local-first Agent Control Plane and Designer.

It is not another multi-agent framework. It manages, connects, observes, and governs agent runtimes, model providers, tools, packs, policies, traces, and artifacts.

Current product goal:

```text
Users should be able to install Nomici, configure an LLM provider,
install a useful agent pack, review permissions, run a real task,
and inspect traces and artifacts.
```

Nomici should serve two audiences:

- Users who want first-run useful agents.
- Developers who want a clean extension architecture.

## Settled Decisions

- Core does not contain heavy Office or Browser runtimes. Core manages Tool Packs.
- AgentGraph IR is internal in v0.1, not a public standard.
- `native_agent` is not a v0.1 public kind. Use `gateway_agent` for the minimal Gateway-run coordinator loop.
- Agent is the atomic extension unit.
- Pack is the distribution and composition unit.
- Agent Pack can install multiple agents, graph edges, tools, policies, examples, and tests.
- Provider setup is a v0.1 core feature.
- Gateway is the only control plane. CLI and Console should go through Gateway APIs once Gateway is running.
- Run Engine stays lightweight. Durable execution belongs to external runtimes.
- Side-effecting tools go through Policy, Approval, and Trace when Nomici mediates execution.
- Pack `official` trust in v0.1 requires a bundled pack or compiled official index. Local manifest claims are not authoritative.
- Console setup requires a running Gateway. v0.1 can use CLI-first setup or a bootstrap `gateway run --setup` mode.
- SQLite is the default local store. Postgres is later.
- v0.1 does not include remote/team/multi-user mode.
- v0.1 targets first-run useful, not full autonomy.

## Application Modules

Console:

```text
Dashboard
Setup
Pack Gallery
Canvas
Agents
Runtimes
Models
Tools
Runs
Traces
Approvals
Artifacts
Settings
```

Gateway:

```text
Gateway Agent Loop
Setup Engine
Provider Registry
Pack Manager
AgentGraph Compiler
Runtime Registry
Runtime Reconciler
Adapter Registry
Tool Broker
Policy Engine
Approval Queue
Run Engine
Task Ledger
Trace Store
Artifact Store
Secrets Resolver
Event Stream
```

Data plane:

```text
LLM Providers
  OpenAI / Anthropic / Gemini / OpenRouter
  Ollama / vLLM / LM Studio / SGLang / llama.cpp

Agent Runtimes
  Hermes
  OpenClaw
  LangGraph
  CrewAI
  OpenAI Agents SDK
  Google ADK
  Agent Squad
  coding-agent fleets

Tool Packs
  Office
  Browser
  Filesystem
  Developer
  Personal Ops
```

## First-Run Flow

The target v0.1 flow:

```text
Install
  -> Doctor
  -> Provider setup
  -> Model test
  -> Pack selection
  -> Permission review
  -> Gateway start
  -> First run
  -> Trace and artifact review
```

CLI shape:

```bash
nomici doctor
nomici model setup
nomici model test
nomici pack list
nomici pack inspect developer-team
nomici pack install developer-team
nomici up
nomici run product_pm "..."
nomici trace show <run_id>
```

Console shape:

- setup checklist
- provider catalog
- capability probe result
- pack gallery
- permission review
- first-run launcher
- trace and artifact viewer

Console setup boundary:

- Console is served by Gateway and cannot run before a Gateway process exists.
- CLI-first setup is the hard v0.1 path.
- A bootstrap Gateway mode can serve setup UI before runtimes are started.

## Extension Model

Atomic extension units:

- Agent Definition
- Tool Definition
- Provider Definition
- Adapter Definition
- Eval Rubric

Composition/distribution units:

- Agent Pack
- Tool Pack
- Provider Pack
- Runtime Adapter Pack
- Template Pack
- Eval Pack

Important distinction:

```text
Agent Definition
  A single reusable agent.

Agent Pack
  A coordinated group of agents, edges, tools, policies, examples, and tests.

Pack
  A general distribution unit for providers, tools, agents, runtimes, templates, and evals.
```

Agent Pack graph edges should use precise coordination modes:

- `delegates_to`, `handoff`, and `agent_as_tool` for internal graph coordination.
- `a2a` for protocol-based interoperability across runtimes, servers, vendors, or remote agents.
- `uses_tool` and `uses_mcp` for tool/data access.
- `requires_approval` for human and policy gates.

Nomici should not label every agent-to-agent edge as A2A. A2A is the interoperability protocol for heterogeneous or remote agents.

## Control Plane Boundary

Nomici owns:

- desired state
- observed state
- registries
- graph compilation
- runtime lifecycle
- adapter invocation
- policy decisions
- approvals
- traces
- artifacts
- pack installation
- provider profiles

Nomici does not own:

- model execution
- durable workflow engines
- framework-native agent loops
- local office document engines
- browser automation engines
- hard sandboxing for malicious local code

Gateway Agent boundary:

Nomici may run a minimal `gateway_agent` loop for first-run coordination. That loop may call a model, request handoffs, call agent-as-tool adapters, and request Nomici-mediated tools. It is not durable execution and should not grow into a full framework runtime.

When a data-plane runtime has a stronger primitive, Nomici integrates it instead of reimplementing it.

## Engineering Architecture

Target Go packages:

```text
internal/gateway
internal/setup
internal/providers
internal/packs
internal/graph
internal/registry
internal/runtime
internal/reconciler
internal/adapters
internal/policy
internal/approvals
internal/tools
internal/runs
internal/trace
internal/artifacts
internal/secrets
```

Target frontend layout:

```text
apps/web/src/
  app/
  api/
  components/
  features/
    setup/
    packs/
    canvas/
    agents/
    runtimes/
    models/
    tools/
    runs/
    traces/
    approvals/
    artifacts/
    settings/
  styles/
```

Gateway API groups:

```text
/api/health
/api/setup
/api/providers
/api/models
/api/packs
/api/graphs
/api/agents
/api/runtimes
/api/tools
/api/runs
/api/tasks
/api/traces
/api/approvals
/api/artifacts
/api/policies
/api/secrets
```

Protocol surfaces:

```text
/v1/*
/a2a/*
/mcp/*
/events
/ws
```

Reserved paths should not imply full compatibility before implementation exists.

`/api/health` is a health-probe exception and may return a minimal naked JSON response. Normal command/query APIs should use the documented response envelope.

## Storage

Default:

- SQLite with WAL.
- Workspace-local `.nomici/state.db`.
- Global profiles under `~/.nomici/`.

Suggested tables:

- `profiles`
- `provider_profiles`
- `pack_installations`
- `graph_snapshots`
- `runtime_desired_state`
- `runtime_observed_state`
- `runs`
- `tasks`
- `trace_events`
- `approvals`
- `artifacts`
- `eval_results`

Rules:

- `nomici.yaml` is Git-friendly.
- `.nomici/` is local state and ignored.
- Provider profiles may be global or project-local.
- Raw secrets are not stored in AgentSpec.

## Technology Taste

Core stack:

- Go for CLI and Gateway.
- React, TypeScript, Vite, React Flow, and pnpm for Console.
- SQLite for local-first state.
- JSON Schema for AgentSpec and pack manifests.
- Makefile for contributor commands.
- Embedded Console assets in release binary.

This stack is intentionally boring in the core and modern in the UI.

Avoid:

- large Go web frameworks
- TypeScript backend for core control plane
- Python as primary Gateway runtime
- bundling heavy office/browser dependencies into core
- premature microservices
- premature marketplace

## Security Defaults

- Gateway binds to `127.0.0.1`.
- Gateway token auth is enabled.
- Remote access is disabled.
- Secrets are referenced, not embedded.
- MCP servers and remote agents are untrusted.
- Side-effecting tools require approval by default.
- Tool invocation is traced and audited.
- Debug bundles and trace exports redact secrets.

See `SECURITY.md` and `docs/security/threat-model.md`.
