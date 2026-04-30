# RFC 0006: Control Plane Architecture

Status: Draft
Date: 2026-04-30
Target release: Nomici Orchestrator v0.1 and beyond

## Summary

Nomici Orchestrator should be engineered as an agent control plane and designer, not as another multi-agent framework.

The project should stand on existing agent runtimes and protocols:

- A2A for agent-to-agent interoperability.
- MCP for tool, data, and workflow integration.
- OpenAI-compatible and native provider APIs for models and agent endpoints.
- LangGraph, Google ADK, OpenAI Agents SDK, CrewAI, Hermes, OpenClaw, and similar systems as data-plane runtimes.

Nomici's differentiated engineering surface is the control plane:

- AgentGraph IR
- desired-state and observed-state reconciliation
- runtime-neutral adapter contracts
- capability manifests
- trust profiles
- policy-aware tool brokering
- event-sourced traces
- approval, audit, and eval loops
- local-first Gateway and Console

## Positioning Decision

Nomici should not compete as:

> another multi-agent framework

Nomici should compete as:

> the open, local-first Agent Control Plane and Designer for agent organizations

The closest product category is an open-source, local-first version of an enterprise agent designer and governance plane:

- visual agent organization design
- root agent and subagent composition
- custom and external agent registration
- runtime lifecycle management
- A2A interoperability
- MCP tool/data connectivity
- policy and approval gates
- trace, replay, eval, and audit
- deployment and governance state

## Why This Matters

The agent ecosystem is already crowded with strong runtimes and frameworks:

- LangGraph provides durable, stateful, long-running workflow execution.
- Google ADK provides parent agents, subagents, workflow agents, and multi-agent composition patterns.
- OpenAI Agents SDK provides agents, tools, handoffs, guardrails, agents-as-tools, sessions, and tracing.
- CrewAI provides crews, flows, role-based agents, stateful workflows, and production-oriented agent automation.
- Agent Squad provides routing and supervisor-agent patterns, including agent-as-tools coordination.
- Hermes and OpenClaw provide strong local agent runtimes and gateway-like surfaces.
- Coding-agent orchestrators provide worktree isolation, PR loops, CI repair, and review dashboards.

Nomici should integrate these capabilities rather than reimplement them.

The unmet open-source opportunity is the layer above them:

```text
design -> register -> run -> connect -> govern -> observe -> evaluate -> deploy
```

## Core Abstractions

### Agent Organization

An agent organization is the user-facing unit of composition.

It contains:

- root agents
- subagents
- external agents
- model providers
- runtimes
- MCP servers
- tools
- memory nodes
- policy gates
- approval gates
- graph edges
- deployment intent

The organization is defined by `nomici.yaml` and compiled into AgentGraph IR.

### AgentGraph IR

AgentGraph IR is the normalized intermediate representation produced from AgentSpec and Web Console edits.

It is not the same thing as the canvas. The canvas is one editor for the graph. `nomici.yaml` is one source format for the graph. AgentGraph IR is the internal contract used by Gateway, policy, runtime management, trace, and deployment.

AgentGraph IR should represent:

- nodes
- edges
- capabilities
- trust profiles
- scopes
- policy attachments
- routing metadata
- runtime bindings
- deployment metadata
- schema version

Node categories:

- `root_agent`
- `subagent`
- `external_agent`
- `remote_a2a_agent`
- `model_provider`
- `runtime`
- `mcp_server`
- `tool_provider`
- `router`
- `approval_gate`
- `policy_gate`
- `memory`
- `webhook`
- `human`

Edge categories:

- `delegates_to`
- `handoff`
- `a2a`
- `agent_as_tool`
- `uses_model`
- `uses_tool`
- `uses_mcp`
- `reads_memory`
- `writes_memory`
- `requires_approval`
- `fallback`
- `parallel`
- `fan_in`
- `deploys_to`

v0.1 does not need to execute every edge category. It should still be able to validate, render, and preserve them.

### Desired State and Observed State

Nomici should use a control-plane model:

```text
AgentSpec / Console edits -> desired state
runtime health / processes / endpoints / traces -> observed state
reconciler -> actions and events
```

Desired state:

- Graph definition
- Runtime definitions
- Model definitions
- Tool definitions
- Policies
- Approval rules
- Deployment intent

Observed state:

- runtime PID
- runtime port
- runtime health
- endpoint availability
- active runs
- recent failures
- adapter capabilities
- trace events
- pending approvals
- policy decisions

The Gateway should reconcile desired and observed state instead of acting as a thin command launcher.

### Runtime Reconciler

The reconciler makes Nomici feel like a real control plane.

Examples:

- Desired runtime is running, observed runtime is missing: start it.
- Desired endpoint should be healthy, observed health check fails: emit warning or restart according to policy.
- Desired port is occupied: fail with a specific remediation.
- Desired adapter capability changed: update registry and notify Console.
- Desired policy changed: recalculate risky routes and approvals.
- Desired graph references a missing runtime: block apply with a clear error.

v0.1 reconciler scope:

- Load desired state from `nomici.yaml`.
- Start local process runtimes.
- Track PID, logs, health, and endpoint.
- Emit runtime events.
- Surface drift in CLI and Console.

Deferred:

- Full self-healing.
- Distributed reconciliation.
- Kubernetes-style controllers.
- Multi-user conflict handling.

### Runtime-Neutral Adapter Contract

Nomici should define a small adapter contract so different runtimes can be registered without becoming first-class framework rewrites.

Minimum v0.1 contract:

- `health`
- `capabilities`
- `invoke`
- `stream`
- `cancel`
- `logs`

Future contract:

- `start`
- `stop`
- `inspect`
- `list_sessions`
- `export_trace`
- `approval_bridge`
- `eval_metadata`
- `artifact_manifest`

Adapter levels:

- Level 1: OpenAI-compatible endpoint.
- Level 2: A2A sidecar.
- Level 3: native runtime adapter.

v0.1 should prioritize Level 1 and only implement enough Level 2 surface to avoid painting the architecture into a corner.

## Protocol Strategy

### Agent to Agent: A2A

Nomici should use A2A for interoperable agent-to-agent communication.

Nomici should provide:

- A2A client
- A2A server
- A2A registry entries
- A2A sidecar pattern for non-A2A runtimes
- A2A trace bridge
- trust profile per remote agent

Nomici should not invent a competing private agent protocol unless required internally, and any internal protocol should be hidden behind adapters.

### Agent to Tool/Data: MCP

Nomici should use MCP for tools, data, and workflow providers.

Nomici should provide:

- MCP server registry
- MCP client
- MCP tool catalog
- MCP permissions
- MCP approval rules
- MCP audit events

MCP servers should be untrusted by default.

### Agent to Model: Provider APIs

Nomici should support:

- OpenAI-compatible endpoints
- provider-native model APIs where needed
- local model providers such as Ollama, vLLM, SGLang, LM Studio, and llama.cpp server

Model provider entries should remain separate from agent endpoint entries. An external agent endpoint may expose an OpenAI-compatible API without being a raw model provider.

## Capability Manifest

Every runtime, agent endpoint, MCP server, and model provider should expose or declare a capability manifest.

Example capability dimensions:

- `streaming`
- `cancel`
- `tools`
- `mcp`
- `a2a`
- `files_read`
- `files_write`
- `shell`
- `browser`
- `network`
- `vision`
- `structured_output`
- `reasoning`
- `memory`
- `artifacts`
- `trace_export`
- `approval_bridge`

Capabilities may be:

- declared by spec
- discovered by adapter
- inferred from endpoint metadata
- manually overridden by user

Capability claims from remote or external systems are not proof. Policy must combine capabilities with trust profile.

## Trust Profile

Each node that can execute or influence execution should have a trust profile.

Trust levels:

- `trusted`
- `untrusted`
- `sandboxed`
- `remote`
- `operator`

Default trust:

- native local Nomici agent: trusted within workspace policy
- local external runtime: untrusted unless configured otherwise
- MCP server: untrusted
- remote A2A agent: untrusted
- OpenAI-compatible external agent endpoint: operator surface
- raw model provider: provider trust depends on config

Trust profile should affect:

- routing
- tool availability
- approval requirements
- secret sharing
- trace redaction
- UI warnings
- remote access restrictions

## Policy-Aware Tool Broker

Tools should not be invoked through uncontrolled paths when Nomici mediates the run.

Preferred path:

```text
agent
  -> Gateway tool broker
  -> policy evaluation
  -> approval queue if needed
  -> MCP/tool/runtime invocation
  -> trace and audit events
```

The broker should know:

- actor
- agent
- runtime
- tool
- MCP server
- arguments summary
- trust profile
- capability manifest
- workspace
- current run
- active policy

v0.1 can implement this as a simple policy gate and approval queue. The architecture should still make the broker the long-term enforcement point.

## Run Engine

Nomici's run engine should coordinate control-plane execution, not replace runtime engines.

v0.1 run engine:

- create run ID
- resolve target agent
- invoke adapter
- stream output when supported
- emit trace events
- apply policy before Nomici-mediated tools
- create approvals
- cancel when supported

Deferred:

- durable workflow execution
- full fan-out/fan-in orchestration
- long-running checkpointed execution
- framework-native state resume

For durable workflow needs, Nomici should prefer LangGraph, ADK, CrewAI Flows, OpenAI Agents SDK sessions, or another specialized runtime as the data plane.

## Trace, Audit, and Eval

Nomici should use event-sourced traces.

Run events should be append-only and structured:

- `run.started`
- `run.completed`
- `run.failed`
- `agent.invoked`
- `agent.completed`
- `agent.failed`
- `model.requested`
- `model.completed`
- `tool.requested`
- `tool.completed`
- `tool.failed`
- `handoff.created`
- `handoff.accepted`
- `approval.requested`
- `approval.granted`
- `approval.denied`
- `runtime.started`
- `runtime.stopped`
- `policy.blocked`
- `budget.exceeded`
- `eval.scored`

Trace enables:

- timeline replay
- audit
- debugging
- cost accounting
- latency analysis
- approval latency analysis
- regression comparison
- eval datasets

v0.1 replay means timeline replay, not deterministic re-execution.

Eval should be built on traces rather than a separate system.

Possible eval dimensions:

- task success
- tool choice quality
- handoff quality
- policy compliance
- cost
- latency
- human approval burden
- error class frequency

## Console Model

Nomici Console should reflect the control-plane model:

- Dashboard: observed health and recent activity.
- Canvas: AgentGraph IR visual editor.
- Agents: registry and configuration.
- Runtimes: desired and observed runtime state.
- Models: provider registry and capabilities.
- Tools/MCP: tool catalog, trust, and permissions.
- Runs/Traces: event timelines and replay.
- Approvals: pending risky actions.
- Evals: trace-derived evaluation views, later.
- Deployments: target environments and drift, later.

The canvas should not be treated as the source of truth. The source of truth is AgentGraph IR derived from AgentSpec and Console edits.

## Competitive Boundary

Nomici should integrate, not clone.

| Existing system | Strength | Nomici boundary |
| --- | --- | --- |
| Gemini Enterprise | Agent Designer, Agent Gallery, governance | Open local-first equivalent, not Google service clone |
| LangGraph | Durable stateful workflows | Use as runtime backend |
| Google ADK | Parent agents, subagents, workflow agents | Register and govern ADK agents |
| OpenAI Agents SDK | Handoffs, agents-as-tools, guardrails, tracing | Bridge and govern SDK agents |
| CrewAI | Crews, flows, role-based automation | Register crews/flows as nodes |
| Agent Squad | Routing, SupervisorAgent, agent-as-tools | Integrate as runtime/backend |
| Hermes | Local agent runtime and gateway | Manage as external runtime |
| OpenClaw | Gateway, operator runtime, Control UI | Manage as external runtime |
| Coding-agent orchestrators | Worktrees, PR/CI loops | Treat as specialized runtime/template |

## v0.1 Architecture Commitments

v0.1 should implement:

- Gateway-centered control plane.
- AgentSpec loading and validation.
- Basic AgentGraph IR representation.
- Model, runtime, and agent registries.
- Local process runtime manager.
- Basic reconciler for configured local runtimes.
- OpenAI-compatible endpoint adapter.
- Hermes and OpenClaw endpoint entries.
- SQLite event store.
- Trace timeline.
- Approval queue.
- Conservative policy defaults.
- Basic Console views.

v0.1 may stub or reserve:

- A2A server/client.
- MCP broker.
- eval views.
- deployment targets.
- adapter capability discovery.
- full sidecar model.

## Non-Goals

Nomici should not:

- implement a full general-purpose multi-agent framework in v0.1
- reimplement LangGraph durable execution
- reimplement ADK subagent engine
- reimplement OpenAI Agents SDK handoffs
- reimplement CrewAI crews and flows
- expose a public cloud control plane by default
- hide security boundaries behind a friendly canvas
- treat remote agent capability claims as trusted facts

## Open Questions

- Should AgentGraph IR be stored in SQLite as normalized tables, JSON blobs, or both?
- Should AgentGraph IR be exposed as a public API in v0.1 or remain internal?
- Should the Console edit AgentSpec directly or edit IR and export AgentSpec?
- Should OpenAI-compatible `/v1/*` route to raw models, agents, or both through explicit prefixes?
- Should the first MCP integration be a brokered registry or direct pass-through?
- Should eval events live in the same trace event table or in a separate eval table?
- Should a local-only Nomici Gateway require token auth from the first dev build?
