# RFC 0009: Application and Engineering Architecture

Status: Draft
Date: 2026-05-01
Target release: Nomici Orchestrator v0.1

## Summary

This RFC consolidates Nomici's application architecture and engineering architecture into one reviewable draft.

Nomici should be:

```text
a local-first Agent Control Plane and Designer
with first-run usable provider setup and agent packs
backed by a clean, extensible engineering core
```

The project should feel useful to users on day one while remaining attractive to contributors who want a serious architecture rather than a demo framework.

## Design Principles

First-run useful:

Users should be able to configure an LLM provider, install a pack, and run a real task without designing an agent organization from scratch.

Control-plane first:

Nomici owns registry, graph, lifecycle, policy, approval, shared context, traces, eval hooks, artifacts, and deployment state. Specialized runtimes own their execution engines and agent-native memory.

Packs over built-in bloat:

Office, browser, developer, personal-ops, and other heavy capabilities should be delivered through packs mediated by Gateway policy, not embedded directly into core.

Internal IR:

AgentGraph IR is an internal contract, not a public standard in v0.1. External extension should happen through AgentSpec, pack manifests, Gateway APIs, and adapter contracts.

Simple production taste:

Prefer boring, reliable infrastructure: one Go binary, embedded Console assets, SQLite by default, JSON Schema for specs, pnpm for frontend workspace, Makefile for contributor commands.

Security by default:

Loopback Gateway, token auth, no raw secrets in specs, approval-first risky tools, trace redaction, untrusted MCP and remote agents.

## Application Architecture

Nomici's product surface should be organized around these modules:

```text
Nomici Console
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

Nomici Gateway owns the control-plane backend:

```text
Nomici Gateway
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
  Shared Context
  Trace Store
  Artifact Store
  Secrets Resolver
  Event Stream
```

Data-plane systems remain external or pack-provided:

```text
LLM Providers
  OpenAI / Anthropic / Gemini / OpenRouter
  Ollama / vLLM / LM Studio / SGLang / llama.cpp

Agent Runtimes
  Claude Code
  Codex
  opencode
  Aider / Cline / Continue
  Hermes
  OpenClaw
  LangGraph
  CrewAI
  OpenAI Agents SDK
  Google ADK
  Agent Squad
  CLI agent fleets

Tool Packs
  Office
  Browser
  Filesystem
  Developer
  Personal Ops
```

## Core User Flow

The primary v0.1 user flow should be:

```text
Install Nomici
  -> nomici doctor
  -> configure provider
  -> test model
  -> choose agent pack
  -> review permissions
  -> start Gateway and runtimes
  -> run task
  -> inspect trace and artifacts
```

This flow should be available from both CLI and Console.

CLI:

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

Console:

- setup checklist
- provider catalog
- capability probe result
- pack gallery
- permission review
- first-run launcher
- trace and artifact viewer

## Control Plane Boundary

Nomici controls:

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

- provider model execution
- durable workflow engines
- framework-native agent loops
- agent-native memory
- local office document engines
- browser automation engines
- arbitrary sandboxing for malicious code

When a data-plane runtime has a stronger primitive, Nomici should integrate it.

Examples:

- Durable workflows: LangGraph, ADK, CrewAI Flows, OpenAI Agents SDK sessions.
- Coding-agent teams: worktree/PR/CI orchestrators.
- Tool/data access: MCP servers.
- Agent-to-agent interop: A2A.

## Application Module Details

### Setup Engine

Responsibilities:

- first-run checklist
- `nomici doctor`
- provider setup wizard
- model test prompt
- capability probe
- local dependency checks
- pack recommendation

Inputs:

- provider catalog
- pack requirements
- local machine checks
- current profile

Outputs:

- model profiles
- provider capability records
- setup warnings
- recommended packs

### Provider Registry

Responsibilities:

- provider catalog
- model profiles
- OpenAI-compatible endpoint config
- local model provider config
- capability metadata
- cost/context metadata
- fallback relationships

Provider registry should distinguish:

- raw model providers
- provider gateways
- external agent endpoints

This distinction prevents an OpenAI-compatible agent endpoint from being treated as a raw model.

### Pack Manager

Responsibilities:

- pack manifest validation
- pack inspection
- permission review
- pack installation
- pack update/removal
- official pack discovery
- local pack development workflow

v0.1 trust root:

- official packs must be bundled with the binary or listed in a compiled official pack index
- local pack manifest claims of `trust.level: official` are not authoritative
- permission review still applies to official packs

Pack types:

- Provider Pack
- Tool Pack
- Agent Pack
- Runtime Adapter Pack
- Template Pack
- Eval Pack

Extension granularity:

- Agent Definition: one reusable agent as an atomic extension.
- Agent Pack: a coordinated team of agents plus graph edges, tools, policies, examples, and tests.
- Pack: the general distribution unit for agent packs, tool packs, provider packs, runtime adapter packs, template packs, and eval packs.

Decision:

> Agent is the atomic extension unit. Pack is the distribution and composition unit.

Agent Pack graph edges should use the right coordination mode:

- `delegates_to`, `handoff`, and `agent_as_tool` for internal graph coordination.
- `a2a` for protocol-based interoperability across runtimes, servers, vendors, or remote agents.
- `uses_tool` and `uses_mcp` for tool/data access.
- `requires_approval` for human and policy gates.

The pack manager should never silently execute untrusted install logic.

### AgentGraph Compiler

Responsibilities:

- compile AgentSpec and Console edits into AgentGraph IR
- validate node and edge references
- apply trust defaults
- attach capability metadata
- attach policy references
- produce graph snapshots

AgentGraph IR is internal in v0.1. Public extension should use AgentSpec and pack manifests.

### Runtime Registry

Responsibilities:

- desired runtime definitions
- observed runtime records
- runtime capabilities
- runtime health
- runtime logs
- runtime endpoints

Runtime records must separate desired state from observed state.

### Runtime Reconciler

Responsibilities:

- compare desired and observed runtime state
- start requested local process runtimes
- stop requested local process runtimes
- detect drift
- run health checks
- emit runtime events
- surface warnings

v0.1 should be conservative. Automatic restart and self-healing require explicit restart policy.

### Adapter Registry

Responsibilities:

- register adapters
- resolve adapter by runtime or endpoint kind
- expose health/capability/invoke/stream/cancel/logs operations
- normalize adapter errors

v0.1 adapters:

- OpenAI-compatible endpoint
- generic CLI Agent Runner
- CLI agent profiles for Codex, Claude Code, opencode, Aider, custom commands, and editor-native agents where they expose automation surfaces
- Hermes endpoint
- OpenClaw endpoint
- Ollama/local provider
- local process runner for lifecycle

### Tool Broker

Responsibilities:

- central tool invocation path
- MCP client integration
- tool permission checks
- approval request creation
- trace/audit events
- artifact registration

All high-risk side effects should go through the broker when Nomici mediates the run.

### Policy Engine

Responsibilities:

- evaluate operation risk
- allow, deny, or require approval
- apply trust profiles
- apply workspace scopes
- apply pack permissions
- emit policy events

Policy should be simple in v0.1 and evolve only when real use cases justify complexity.

### Approval Queue

Responsibilities:

- pending approvals
- grant/deny decisions
- risk summary
- diff preview where possible
- approval audit log

Approval scopes:

- once
- run
- session

Longer-lived approvals should be deferred until policy is more mature.

### Run Engine

Responsibilities:

- create run ID
- resolve target agent
- invoke adapter or `gateway_agent`
- stream output
- apply policy before Nomici-mediated tools
- emit trace events
- handle cancellation

The run engine should remain thin. It coordinates control-plane execution; it does not become a durable workflow engine in v0.1.

`gateway_agent` is the v0.1 name for the minimal Gateway-run coordinator loop. `native_agent` is not a public v0.1 kind. The first compelling developer proof may use external `cli_agent` runtimes before the Gateway agent loop is featureful.

### Shared Context

Responsibilities:

- project context
- run context
- handoff briefings
- context snapshots
- open issues
- user feedback
- artifact summaries

Shared Context is not a replacement for Hermes/OpenClaw memory, coding-agent sessions, LangGraph state, or framework-native memory. It is a control-plane bridge that gives downstream agents useful briefing context without requiring raw memory dumps.

### Task Ledger

Responsibilities:

- long-running task records
- subtasks
- assignments
- checkpoints
- artifacts
- approvals
- resume/cancel metadata

This gives users a durable view of long-running work without forcing Nomici to implement a full workflow runtime immediately.

### Trace Store

Responsibilities:

- append-only trace events
- run summaries
- JSONL export
- redaction metadata
- eval hook input

Trace is the foundation for debugging, replay timeline, audit, and eval.

### Artifact Store

Responsibilities:

- generated files
- tool outputs
- document exports
- screenshots
- patches
- metadata
- redaction markers

Artifacts should live under workspace-local `.nomici/` by default.

### Secrets Resolver

Responsibilities:

- env var references
- keychain integration later
- redaction
- no raw secrets in public API responses

Secrets resolver should be used by providers, adapters, packs, and tools.

## Engineering Architecture

Recommended repository layout:

```text
cmd/
  nomici/

internal/
  gateway/
  setup/
  providers/
  packs/
  graph/
  registry/
  runtime/
  reconciler/
  adapters/
  policy/
  approvals/
  tools/
  runs/
  context/
  trace/
  artifacts/
  secrets/

apps/
  web/

packages/
  spec/
  client-js/

packs/
  official/

examples/
docs/
scripts/
```

This is a target structure. v0.1 implementation can start smaller and split packages when responsibilities become real.

## Go Package Boundaries

### `internal/gateway`

Owns:

- HTTP server
- routing
- REST API
- WebSocket/SSE event stream
- embedded Console

Should not own:

- business logic for provider setup
- runtime reconciliation
- pack installation
- policy logic

Gateway should call services, not become a large god package.

### `internal/setup`

Owns:

- setup checklist
- doctor checks
- provider setup orchestration
- pack recommendation

### `internal/providers`

Owns:

- provider catalog
- model profile types
- capability probes
- provider health/test prompt

### `internal/packs`

Owns:

- pack manifest schema
- pack validation
- install/inspect/update/remove
- permission summaries

### `internal/graph`

Owns:

- AgentGraph IR types
- AgentSpec compiler
- graph validation
- graph snapshots

### `internal/runtime`

Owns:

- local process runner
- PID/log handling
- health check execution
- runtime observed state helpers

### `internal/reconciler`

Owns:

- desired vs observed comparison
- reconcile actions
- drift detection
- runtime lifecycle decisions

### `internal/adapters`

Owns:

- adapter interface
- adapter registry
- endpoint adapters
- normalized adapter errors

### `internal/policy`

Owns:

- policy evaluation
- risk classification
- allow/deny/approval decisions

### `internal/tools`

Owns:

- tool broker
- MCP client later
- tool invocation audit

### `internal/runs`

Owns:

- run engine
- task ledger
- cancellation

### `internal/context`

Owns:

- shared context items
- handoff context snapshots
- task briefings
- context redaction
- promotion from trace/artifacts into Shared Context

### `internal/trace`

Owns:

- trace event types
- event store
- JSONL export
- eval event persistence hooks

### `internal/artifacts`

Owns:

- artifact metadata
- workspace artifact paths
- artifact redaction metadata

### `internal/secrets`

Owns:

- secret references
- env resolver
- keychain resolver later
- redaction

## Frontend Architecture

Recommended app layout:

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

Frontend rules:

- Console calls Gateway APIs.
- Console does not contain control-plane business logic.
- Console never receives raw secret values.
- React Flow canvas renders AgentGraph-derived view models, not raw internal storage rows.
- Setup and Pack Gallery are first-class screens, not hidden settings pages.

## API Shape

Gateway APIs should be grouped by product module:

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
/api/context
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

v0.1 should implement only what is needed. Reserved paths should not imply full compatibility before it exists.

`/api/health` is the liveness/readiness exception and may return minimal naked JSON. Normal command/query APIs should use the standard response envelope.

## Storage Architecture

Default local storage:

- SQLite with WAL
- workspace-local state by default
- global profile config for reusable provider profiles and secrets references

Suggested SQLite tables:

- `profiles`
- `provider_profiles`
- `pack_installations`
- `graph_snapshots`
- `runtime_desired_state`
- `runtime_observed_state`
- `runs`
- `tasks`
- `context_items`
- `context_snapshots`
- `trace_events`
- `approvals`
- `artifacts`
- `eval_results`

Files:

```text
project/
  nomici.yaml
  .nomici/
    state.db
    runs/
    artifacts/
    context/
    logs/
```

Global:

```text
~/.nomici/
  config.yaml
  profiles/
  secrets/
  cache/
```

Rules:

- `nomici.yaml` is Git-friendly.
- `.nomici/` is local state and ignored.
- Provider profiles may be global or project-local.
- Raw secrets are not stored in AgentSpec.

## Technology Stack Taste

Recommended stack remains:

- Go for CLI and Gateway.
- React, TypeScript, Vite, React Flow, and pnpm for Console.
- SQLite by default.
- JSON Schema for AgentSpec and pack manifests.
- Makefile as contributor command surface.
- Embedded Console assets in release binary.

This stack is intentionally boring in the core and modern in the UI.

Reasons:

- Go keeps distribution simple.
- React Flow avoids rebuilding graph UI.
- pnpm workspace is standard for TS package organization.
- SQLite is excellent for local-first state.
- JSON Schema gives editor autocomplete and validation.
- Makefile hides multi-language build complexity.

Avoid:

- a large Go web framework
- a full TypeScript backend for the control plane
- Python as the primary Gateway runtime
- bundling office/browser heavy dependencies into core
- premature microservices
- premature plugin marketplace

## v0.1 Implementation Phases

Before the full product slice, implementation should prove:

- provider setup
- OpenAI-compatible or Ollama model profile
- one Gateway-mediated invocation
- trace event storage
- secret redaction
- health/status visibility

The developer proof slice should also prove:

- `cli_agent` external invocation
- workspace diff capture
- handoff context snapshot
- approval gate before publish actions

This proof slice should not require packs, full graph canvas editing, A2A sidecars, or MCP proxying.

Phase 0: Foundation

- repo governance
- RFCs
- CLI/Gateway scaffold
- Console scaffold
- health endpoint

Phase 1: Provider Setup

- provider catalog scaffold
- generic OpenAI-compatible setup
- Ollama setup
- model test prompt
- capability profile

Phase 2: AgentGraph and Packs

- pack manifest schema
- AgentSpec to AgentGraph compile
- `ai-application-pm` pack
- `developer-team` pack scaffold
- permission review
- minimal IR fields driven by implemented adapters

Phase 3: Runtime and Adapters

- local process runtime manager
- runtime observed state
- OpenAI-compatible adapter
- generic CLI Agent Runner
- CLI agent profiles for Codex, Claude Code, opencode, Aider, custom commands, and editor-native agents where they expose automation surfaces
- Hermes endpoint entry
- OpenClaw endpoint entry
- health/logs

Phase 4: Runs, Trace, Approval

- run engine
- shared context
- handoff context snapshots
- trace event store
- approval queue
- trace export
- artifact metadata

Phase 5: Console MVP

- setup inspection after Gateway starts
- pack gallery
- canvas view
- models
- runtimes
- runs/traces
- approvals
- artifacts

Full provider setup in Console requires bootstrap Gateway mode. If bootstrap mode is not implemented in the first v0.1 slice, CLI-first setup is the supported path.

## What To Cut If Scope Grows

Cut first:

- remote pack registry
- signed pack distribution
- full office pack
- browser automation
- PR creation
- A2A sidecars
- MCP proxy
- automated eval scoring
- Docker runner

Do not cut:

- provider setup
- trace event store
- security defaults
- model profile storage without raw secrets
- one Gateway-mediated invocation path

Do not cut from the v0.1 product slice:

- pack manifest
- AgentGraph compiler
- runtime observed state
- approval model
- first-run useful official bundled pack

## Open Questions

- Should official bundled packs live in the main repository during v0.1?
- Should `nomici model setup` default to global profile config or project config?
- Should `developer-team` be the default first-run pack?
- Should `office` be official but experimental in v0.1?
- Should Gateway APIs be designed REST-first or event-first for Console?
- Should SQLite schema be introduced before or after AgentGraph compiler?
- Should implementation keep current package skeleton or reorganize before adding behavior?
