# System Composition Design

## Purpose

This document checks whether the major Nomici subsystems compose into a coherent product and engineering architecture.

Nomici should feel like a useful local product after installation, while remaining a serious open-source control plane that developers can extend without being forced into one agent framework.

## Planes

Nomici is split into five planes:

```text
Experience Plane
  CLI
  Console

Definition Plane
  AgentSpec
  Provider profiles
  Pack manifests
  Policy definitions

Control Plane
  Gateway
  registries
  compiler
  reconciler
  run engine
  policy and approval

Execution Plane
  model providers
  agent runtimes
  MCP servers
  tool packs
  adapters

Evidence Plane
  traces
  logs
  approvals
  artifacts
  eval events
  audit exports
```

Rules:

- The Experience Plane never coordinates state directly.
- The Gateway is the only Control Plane process.
- The Definition Plane is Git-friendly and should avoid secrets.
- The Execution Plane performs model calls, agent loops, and tool actions.
- The Evidence Plane records what happened and supports review, replay, export, and eval.
- A minimal `gateway_agent` loop may exist in the Control Plane for first-run coordination, but advanced agent execution belongs in the Execution Plane.

## Dependency Direction

Allowed dependencies:

```text
CLI / Console
  -> Gateway API

Gateway API
  -> setup / packs / graph / runtime / runs / tools / policy / trace services

Provider Setup
  -> provider profiles
  -> capability profiles

Pack Manager
  -> pack manifests
  -> AgentSpec fragments
  -> permission summaries

AgentGraph Compiler
  -> AgentSpec fragments
  -> provider profiles
  -> pack installs
  -> graph snapshots

Runtime Reconciler
  -> desired runtime state
  -> observed runtime state
  -> runtime events

Run Engine
  -> graph snapshots
  -> adapters
  -> tool broker
  -> task ledger
  -> trace store

Tool Broker
  -> policy engine
  -> approval queue
  -> tool packs / MCP servers
  -> artifact store
  -> trace store
```

Disallowed dependencies:

- Console directly reads SQLite.
- CLI mutates runtime state behind Gateway after Gateway is running.
- Tool packs bypass Policy for side effects.
- Runtimes write trace events without Gateway mediation unless through a trace adapter.
- Pack install runs arbitrary setup scripts without explicit approval.
- AgentGraph IR becomes a public contract in v0.1.
- Local pack manifest trust claims grant official privileges.

## Core Data Flow

The first useful vertical slice should be:

```text
nomici model setup
  -> provider profile
  -> model capability profile

nomici pack install developer-team
  -> permission review
  -> AgentSpec fragment
  -> pack installation record

nomici up
  -> Gateway start
  -> graph compile
  -> runtime desired state
  -> runtime observed state

nomici run product_pm "..."
  -> run record
  -> graph snapshot
  -> adapter invocation
  -> tool broker for side effects
  -> trace events
  -> approvals and artifacts
```

This slice proves the product without requiring full A2A, full MCP proxying, durable workflow execution, team mode, or marketplace infrastructure.

A smaller proof slice should land before this:

```text
provider setup
  -> one Gateway-mediated model or external endpoint invocation
  -> trace events
  -> redaction and health/status checks
```

## Why The Pieces Fit

Provider setup is a product feature and a platform primitive.

It makes first-run useful for users and creates structured model capability data for packs, graph validation, fallback selection, and diagnostics.

Packs are the right extension boundary.

They let Nomici ship useful agents without bloating core, while giving outside developers a concrete unit for sharing agents, teams, tools, templates, evals, and adapters.

AgentGraph IR should stay internal.

It lets the compiler normalize packs, AgentSpec, and Console edits into one execution view, but avoids prematurely asking the ecosystem to adopt another standard.

Runtime Reconciler keeps Nomici honest as a control plane.

Instead of merely launching processes, it compares desired and observed state, records drift, and exposes runtime health to CLI, Console, traces, and doctor checks.

Run Engine should stay intentionally small.

It coordinates graph snapshots, adapter calls, tasks, traces, approvals, and artifacts. Durable state machines remain the responsibility of LangGraph, OpenAI Agents SDK, CrewAI, ADK, Hermes, OpenClaw, or other runtimes.

Gateway Agent Loop should be intentionally minimal.

It exists so first-run packs can have a root coordinator without requiring users to install another framework. It should not provide durable execution, framework-native memory, complex planners, or independent side-effect permissions.

Policy and Tool Broker are the safety choke point.

Office files, browser actions, shell, filesystem writes, MCP tools, email, calendar, deploy, and external operator runtimes all converge through one decision path.

Trace Store is the evidence layer.

It gives the project replay, audit, debug bundles, eval hooks, and user trust without requiring deterministic re-execution in v0.1.

SQLite is the right local default.

It matches local-first installation, simple backup, low operational burden, and future Postgres migration through service boundaries.

## Reference Project Lessons

Nomici should borrow patterns without cloning any one project.

| Reference | Useful lesson | Nomici interpretation |
| --- | --- | --- |
| Gemini Enterprise Agent Designer | Visual designer, root agent, subagents, tools, preview, governance | Product shape: open local control plane and designer |
| Agent Squad | Supervisor/routing and agent-as-tools patterns | Adapter/runtime inspiration, not the whole product |
| LangGraph | Durable, stateful, long-running workflows | External runtime backend for workflows needing durability |
| CrewAI | Role-based teams and task collaboration | Pack templates for role/team agent setups |
| OpenAI Agents SDK | Handoffs, agents-as-tools, guardrails, sessions, tracing | Coordination semantics and adapter concepts |
| Composio agent-orchestrator | Coding-agent fleets, worktrees, PR supervision | Future developer-team pack and coding runtime integration |
| AI-Agents-Orchestrator style projects | Multi-CLI assistant supervision | Possible pack/runtime category, not core identity |
| Hermes / OpenClaw | Strong local operator agents with gateways/profiles | Managed external runtimes and endpoint adapters |
| A2A | Cross-vendor agent interoperability | Protocol boundary for heterogeneous/remote agents |
| MCP | Tool/data connection protocol | Tool boundary, mediated by Policy and Tool Broker |

## Tension Points

First-run useful vs clean architecture:

- Ship official packs, provider setup, and guided flows early.
- Keep heavy runtimes out of core.
- Make pack and provider contracts clean enough that useful defaults do not become hard-coded product hacks.

Local productivity vs safety:

- Office/browser/developer tools are important for real work.
- They must enter as Tool Packs with permissions, approvals, artifacts, and trace events.
- Core should not silently gain broad filesystem, shell, browser, or email power.

Visual canvas vs executable graph:

- Console canvas edits are user experience.
- AgentGraph Compiler is the executable authority.
- Every visual change must compile to a validated graph snapshot before execution.

A2A everywhere vs precise semantics:

- Internal edges can be `handoff`, `delegates_to`, `agent_as_tool`, `parallel`, or `fallback`.
- A2A is used when interoperability across runtime, server, vendor, or remote boundary matters.
- This avoids pretending every local coordination edge is a network protocol edge.

Gateway as single control plane vs performance:

- v0.1 favors simple coordination and auditability over distributed throughput.
- Heavy execution stays in the data plane.
- Future remote workers can be added without splitting the local control plane prematurely.

## Invariants

The following should remain true through v0.1:

- Gateway is the only control plane.
- CLI and Console use the same APIs once Gateway is running.
- User-facing configs do not store raw secrets.
- Pack install always has a permission review.
- Side-effecting tools pass through Policy and Approval.
- Every run produces trace events.
- Every graph execution uses an immutable graph snapshot.
- Runtime desired state and observed state are separate.
- AgentGraph IR is internal.
- Durable workflow execution is delegated to external runtimes.
- `official` pack trust requires a bundled pack or compiled official index until signatures exist.
- Console setup requires Gateway; if bootstrap mode is absent, setup is CLI-first.
- Remote/team/multi-user features are deferred.

## Failure Boundaries

Provider setup failure:

- Should not corrupt existing profiles.
- Should emit setup/test events without exposing secrets.

Pack install failure:

- Should not partially activate graph fragments.
- Should leave an installation record with failure reason.

Graph compile failure:

- Should block execution.
- Should point to AgentSpec, pack, or Console source location.

Runtime failure:

- Should not erase desired state.
- Should update observed state and trace events.

Tool approval denial:

- Should resume the run with a denied decision where possible.
- Should preserve the denial in audit history.

Storage failure:

- Should stop Gateway startup if migrations fail.
- Should never silently drop trace or approval data.

## Implementation Order

Recommended order:

1. Gateway API skeleton and storage migration runner.
2. Provider catalog and provider profile storage.
3. Pack manifest validation and permission summary.
4. AgentGraph compile and immutable snapshot.
5. Runtime desired/observed state and health checks.
6. Run engine with trace events.
7. Policy and approval queue for tool calls.
8. Artifact registration.
9. Console pages for setup, packs, runtime status, traces, approvals, and artifacts.

This order preserves the dependency direction and lets each layer prove itself before the next layer depends on it.
