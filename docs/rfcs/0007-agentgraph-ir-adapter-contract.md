# RFC 0007: AgentGraph IR and Adapter Contract

Status: Draft
Date: 2026-04-30
Target release: Nomici Orchestrator v0.1

## Summary

This RFC defines the first implementation-level contracts behind Nomici's control-plane architecture:

- AgentGraph IR schema
- Adapter Contract v0.1
- Runtime Reconciler state model
- Trace Event Schema and eval hooks

The goal is to make Nomici implementable without becoming another multi-agent framework.

Nomici should compile user intent from `nomici.yaml` and Nomici Console into a normalized AgentGraph IR. Gateway then reconciles desired state with observed state, invokes runtimes through adapters, routes tool calls through policy, and records event-sourced traces that can later power eval.

## Design Goals

- Keep the core data model runtime-neutral.
- Support root agents, subagents, external runtimes, models, tools, MCP servers, A2A agents, policy gates, and approval gates.
- Distinguish desired state from observed state.
- Make adapters small enough to implement quickly.
- Keep trace events structured and append-only.
- Leave room for LangGraph, CrewAI, Google ADK, OpenAI Agents SDK, Hermes, OpenClaw, and coding-agent fleets as data-plane runtimes.
- Avoid encoding framework-specific concepts into the core IR unless they generalize.

## Non-Goals

v0.1 does not need:

- A fully durable workflow engine.
- Full A2A server/client implementation.
- Full MCP proxy implementation.
- Deep native adapters for every framework.
- Distributed reconciliation.
- Multi-user conflict resolution.
- Deterministic replay of arbitrary external LLM runs.

## AgentGraph IR

AgentGraph IR is the normalized internal representation used by Gateway.

Sources:

- `nomici.yaml`
- Nomici Console edits
- templates
- future API clients

Consumers:

- Gateway registry
- runtime reconciler
- run engine
- policy engine
- approval queue
- trace store
- Console graph renderer
- deployment planner

AgentGraph IR is not the same thing as React Flow nodes. The Console may render IR, but IR must stay stable enough for CLI, API, validation, and future deployment.

### IR Convergence Policy

This RFC describes the target internal shape. Implementation should not freeze every field before the first adapter runs.

Rules:

- Start with the smallest immutable graph snapshot needed for the proof slice.
- Implement the OpenAI-compatible adapter first and let streaming, cancellation, errors, auth, and trace requirements test the IR.
- Add IR fields only when an adapter, pack, policy check, trace view, or Console feature needs them.
- Keep source mapping and snapshot immutability early because they support errors, audit, and replay.
- Do not expose AgentGraph IR as a public standard in v0.1.

## IR Document Shape

```json
{
  "schema_version": "0.1",
  "graph_id": "ai-application-pm",
  "project": {
    "name": "ai-application-pm",
    "description": "AI application product manager with architecture and UX subagents."
  },
  "nodes": [],
  "edges": [],
  "policies": [],
  "budgets": [],
  "deployment": {},
  "metadata": {}
}
```

Required fields:

- `schema_version`
- `graph_id`
- `project`
- `nodes`
- `edges`

Optional fields:

- `policies`
- `budgets`
- `deployment`
- `metadata`

Rules:

- IR IDs are stable within a graph.
- IR IDs are strings, not database IDs.
- IR should preserve enough source metadata to produce useful validation errors.
- IR must not contain raw secrets.

## IR Node Schema

```json
{
  "id": "product_pm",
  "kind": "root_agent",
  "label": "Product PM",
  "source": {
    "kind": "agentspec",
    "path": "agents.product_pm"
  },
  "spec": {},
  "capabilities": {},
  "trust": {},
  "scopes": {},
  "bindings": {},
  "policy_refs": [],
  "metadata": {}
}
```

Required fields:

- `id`
- `kind`
- `spec`

Recommended fields:

- `label`
- `source`
- `capabilities`
- `trust`
- `scopes`
- `bindings`
- `policy_refs`
- `metadata`

Node kinds:

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

## Node Spec By Kind

### Root Agent

```json
{
  "id": "product_pm",
  "kind": "root_agent",
  "spec": {
    "model_ref": "gpt",
    "role": "Coordinate architecture, UX, and implementation work.",
    "instructions": "",
    "subagent_refs": ["senior_architect", "uiux_designer"],
    "tool_refs": [],
    "runtime_ref": null
  }
}
```

Root agents are entry points for user runs. v0.1 may support only simple native invocation or delegation to an adapter-backed endpoint.

### Subagent

```json
{
  "id": "senior_architect",
  "kind": "subagent",
  "spec": {
    "model_ref": "local_qwen",
    "role": "Design software architecture.",
    "instructions": "",
    "tool_refs": []
  }
}
```

Subagents may be native, model-backed, or adapter-backed.

### External Agent

```json
{
  "id": "hermes_coder",
  "kind": "external_agent",
  "spec": {
    "runtime_ref": "hermes_coder",
    "adapter_ref": "hermes_coder",
    "endpoint_ref": "hermes_coder"
  }
}
```

External agents are controlled by another runtime. Nomici observes and invokes through adapters.

### Runtime

```json
{
  "id": "hermes_coder",
  "kind": "runtime",
  "spec": {
    "runtime_kind": "hermes",
    "runner": "local_process",
    "workspace": "./workspaces/hermes-coder",
    "start": {
      "command": ["hermes", "-p", "coder", "gateway", "run"]
    },
    "api": {
      "kind": "openai_compatible",
      "base_url": "http://127.0.0.1:8642/v1",
      "api_key_env": "HERMES_API_KEY"
    },
    "health": {
      "kind": "http",
      "url": "http://127.0.0.1:8642/health"
    }
  }
}
```

Runtime nodes describe data-plane execution environments.

### Model Provider

```json
{
  "id": "local_qwen",
  "kind": "model_provider",
  "spec": {
    "provider_kind": "ollama",
    "base_url": "http://127.0.0.1:11434",
    "model": "qwen3:32b",
    "api_key_env": null
  }
}
```

Model providers are raw model access. They are separate from external agent endpoints even when both use OpenAI-compatible HTTP.

### MCP Server

```json
{
  "id": "filesystem",
  "kind": "mcp_server",
  "spec": {
    "transport": "stdio",
    "command": ["npx", "@modelcontextprotocol/server-filesystem", "./workspace"]
  }
}
```

MCP servers are tool/data providers. Default trust is `untrusted`.

### Approval Gate

```json
{
  "id": "approval_shell",
  "kind": "approval_gate",
  "spec": {
    "actions": ["tool.shell.exec"],
    "default_decision": "approval"
  }
}
```

Approval gates make risky actions visible in graph and policy evaluation.

## IR Edge Schema

```json
{
  "id": "product_pm-to-hermes_coder",
  "from": "product_pm",
  "to": "hermes_coder",
  "kind": "a2a",
  "mode": "task",
  "conditions": [],
  "policy_refs": [],
  "metadata": {}
}
```

Required fields:

- `id`
- `from`
- `to`
- `kind`

Edge kinds:

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

Rules:

- `from` and `to` must reference existing node IDs.
- v0.1 may render unsupported edge kinds but must fail clearly if a run tries to execute them.
- The compiler should generate implicit `uses_model`, `uses_runtime`, and `requires_approval` edges where useful for policy and visualization.

## Capability Manifest

Every executable or callable node should have capabilities.

```json
{
  "streaming": true,
  "cancel": true,
  "tools": true,
  "mcp": false,
  "a2a": false,
  "files_read": false,
  "files_write": false,
  "shell": false,
  "browser": false,
  "network": true,
  "vision": false,
  "structured_output": true,
  "reasoning": true,
  "memory": false,
  "artifacts": true,
  "trace_export": false,
  "approval_bridge": false
}
```

Capability sources:

- `declared`: user or template declared it
- `discovered`: adapter discovered it
- `inferred`: Nomici inferred it
- `override`: user overrode it

Capability claims from remote or external systems are not trusted facts. Policy must combine capabilities with trust profile.

## Trust Profile

```json
{
  "level": "untrusted",
  "reason": "external runtime endpoint",
  "secret_sharing": "deny",
  "tool_access": "approval",
  "network_access": "approval",
  "filesystem_access": "scoped",
  "audit_level": "full"
}
```

Trust levels:

- `trusted`
- `untrusted`
- `sandboxed`
- `remote`
- `operator`

Default trust:

- native local Nomici agent: `trusted`
- local external runtime: `untrusted`
- MCP server: `untrusted`
- remote A2A agent: `untrusted`
- OpenAI-compatible external agent endpoint: `operator`
- model provider: provider-specific, usually `remote` for cloud and `trusted` or `untrusted` for local

## Adapter Contract v0.1

Adapters connect Nomici to data-plane systems.

v0.1 adapter goals:

- Keep implementation small.
- Support HTTP endpoint adapters first.
- Support local process observation where runtime manager starts the process.
- Return structured capabilities and errors.
- Emit trace events through Gateway, not directly to storage.

## Adapter Interface

Conceptual Go interface:

```go
type Adapter interface {
    ID() string
    Kind() string
    Health(ctx context.Context, target Target) (HealthResult, error)
    Capabilities(ctx context.Context, target Target) (CapabilityManifest, error)
    Invoke(ctx context.Context, request InvokeRequest) (*InvokeResult, error)
    Stream(ctx context.Context, request InvokeRequest) (EventStream, error)
    Cancel(ctx context.Context, request CancelRequest) error
    Logs(ctx context.Context, request LogsRequest) (LogStream, error)
}
```

v0.1 adapters:

- `openai_compatible`
- `hermes_endpoint`
- `openclaw_endpoint`
- `ollama`
- `local_process` for runtime lifecycle, not agent invocation

## Adapter Request Types

### Target

```json
{
  "runtime_id": "hermes_coder",
  "endpoint": "http://127.0.0.1:8642/v1",
  "auth": {
    "kind": "env",
    "name": "HERMES_API_KEY"
  },
  "metadata": {}
}
```

Rules:

- Adapter requests receive secret references or resolved secrets only inside Gateway-controlled execution.
- Resolved secrets must not be returned in adapter results.

### InvokeRequest

```json
{
  "run_id": "run_01H...",
  "node_id": "hermes_coder",
  "target": {},
  "input": {
    "messages": [
      {
        "role": "user",
        "content": "Implement the scaffold."
      }
    ]
  },
  "options": {
    "stream": true,
    "timeout_ms": 120000
  },
  "trace_context": {
    "parent_event_id": "evt_01H..."
  }
}
```

### InvokeResult

```json
{
  "status": "completed",
  "output": {
    "messages": []
  },
  "usage": {
    "input_tokens": 100,
    "output_tokens": 200,
    "cost_usd": null
  },
  "artifacts": [],
  "raw_ref": null
}
```

Statuses:

- `completed`
- `failed`
- `cancelled`
- `requires_approval`
- `unsupported`

### HealthResult

```json
{
  "status": "healthy",
  "message": "endpoint responded",
  "checked_at": "2026-04-30T12:00:00Z",
  "latency_ms": 20,
  "details": {}
}
```

Health statuses:

- `unknown`
- `healthy`
- `degraded`
- `unhealthy`
- `stopped`

### Adapter Error

```json
{
  "code": "endpoint_unavailable",
  "message": "Hermes endpoint did not respond",
  "retryable": true,
  "safe_message": "Endpoint did not respond.",
  "details": {}
}
```

Error codes should be stable enough for UI and policy:

- `unsupported`
- `invalid_config`
- `auth_failed`
- `endpoint_unavailable`
- `timeout`
- `rate_limited`
- `cancelled`
- `policy_blocked`
- `approval_required`
- `runtime_failed`
- `unknown`

## Streaming Contract

Adapters that support streaming should produce normalized events:

- `message.delta`
- `message.completed`
- `tool.requested`
- `tool.completed`
- `tool.failed`
- `usage.updated`
- `artifact.created`
- `adapter.warning`
- `adapter.completed`
- `adapter.failed`

Raw provider events may be attached by reference or redacted metadata, but should not become the public Gateway contract in v0.1.

## Runtime Reconciler State Model

The reconciler compares desired runtime state and observed runtime state.

### Desired Runtime

```json
{
  "runtime_id": "hermes_coder",
  "desired_phase": "running",
  "runner": "local_process",
  "start_command": ["hermes", "-p", "coder", "gateway", "run"],
  "workspace": "./workspaces/hermes-coder",
  "env_refs": ["HERMES_API_KEY"],
  "health_check": {
    "kind": "http",
    "url": "http://127.0.0.1:8642/health"
  },
  "restart_policy": {
    "kind": "on_failure",
    "max_restarts": 3
  }
}
```

Desired phases:

- `running`
- `stopped`
- `disabled`

### Observed Runtime

```json
{
  "runtime_id": "hermes_coder",
  "observed_phase": "running",
  "pid": 12345,
  "endpoint": "http://127.0.0.1:8642/v1",
  "health": {
    "status": "healthy",
    "checked_at": "2026-04-30T12:00:00Z"
  },
  "started_at": "2026-04-30T11:55:00Z",
  "restart_count": 0,
  "last_error": null,
  "capabilities": {}
}
```

Observed phases:

- `unknown`
- `starting`
- `running`
- `degraded`
- `stopping`
- `stopped`
- `failed`

### ReconcileResult

```json
{
  "runtime_id": "hermes_coder",
  "action": "none",
  "reason": "runtime matches desired state",
  "events": []
}
```

Actions:

- `none`
- `start`
- `stop`
- `restart`
- `health_check`
- `mark_degraded`
- `mark_failed`
- `block`

v0.1 reconciler loop:

1. Load desired runtimes from AgentGraph IR.
2. Load observed runtime records from SQLite.
3. Inspect local process state.
4. Run health checks.
5. Emit events.
6. Apply safe actions.
7. Surface drift in CLI and Console.

Safe automatic actions in v0.1:

- health check
- mark degraded
- mark failed
- start runtimes explicitly requested by `nomici up`
- stop runtimes explicitly requested by `nomici down`

Automatic restart should be conservative and controlled by restart policy.

## Trace Event Schema

Trace events are append-only records.

```json
{
  "event_id": "evt_01H...",
  "run_id": "run_01H...",
  "parent_event_id": null,
  "sequence": 1,
  "time": "2026-04-30T12:00:00Z",
  "type": "run.started",
  "actor": {
    "kind": "user",
    "id": "local"
  },
  "node_id": "product_pm",
  "runtime_id": null,
  "trace_context": {},
  "payload": {},
  "redactions": [],
  "metadata": {}
}
```

Required fields:

- `event_id`
- `run_id`
- `sequence`
- `time`
- `type`
- `payload`

Recommended fields:

- `parent_event_id`
- `actor`
- `node_id`
- `runtime_id`
- `trace_context`
- `redactions`
- `metadata`

Rules:

- Event IDs are globally unique.
- Sequence is monotonically increasing within a run.
- Payload must be JSON.
- Secrets must be redacted before export.
- Raw provider payloads should be stored only if policy allows.

## Core Event Types

Run lifecycle:

- `run.started`
- `run.completed`
- `run.failed`
- `run.cancelled`

Agent:

- `agent.invoked`
- `agent.completed`
- `agent.failed`

Adapter:

- `adapter.health_checked`
- `adapter.capabilities_discovered`
- `adapter.invoked`
- `adapter.completed`
- `adapter.failed`
- `adapter.cancelled`

Model:

- `model.requested`
- `model.completed`
- `model.failed`

Tool:

- `tool.requested`
- `tool.completed`
- `tool.failed`

Handoff and A2A:

- `handoff.created`
- `handoff.accepted`
- `handoff.rejected`
- `a2a.task.created`
- `a2a.task.updated`
- `a2a.task.completed`
- `a2a.task.failed`

Policy and approval:

- `policy.allowed`
- `policy.blocked`
- `policy.approval_required`
- `approval.requested`
- `approval.granted`
- `approval.denied`
- `approval.expired`

Runtime:

- `runtime.desired`
- `runtime.started`
- `runtime.stopped`
- `runtime.failed`
- `runtime.health_changed`
- `runtime.reconciled`

Budget and eval:

- `budget.exceeded`
- `eval.requested`
- `eval.scored`
- `eval.failed`

## Eval Hooks

Eval should build on trace events rather than a separate execution system.

An eval hook is a subscriber that can read completed run traces and produce eval events.

Eval hook input:

```json
{
  "run_id": "run_01H...",
  "graph_id": "ai-application-pm",
  "trace_events": [],
  "rubric_ref": "default.task_success",
  "metadata": {}
}
```

Eval hook output:

```json
{
  "eval_id": "eval_01H...",
  "run_id": "run_01H...",
  "scores": [
    {
      "name": "task_success",
      "value": 0.8,
      "scale": "0_1",
      "reason": "Plan covered architecture and UX but lacked deployment details."
    }
  ],
  "labels": ["needs_followup"],
  "metadata": {}
}
```

Initial eval dimensions:

- `task_success`
- `handoff_quality`
- `tool_choice_quality`
- `policy_compliance`
- `approval_burden`
- `latency`
- `cost`
- `error_class`

v0.1 may only store eval events and provide export hooks. Automated scoring can be deferred.

## SQLite Storage Model

v0.1 storage can use a pragmatic hybrid:

- normalized tables for runs, runtime state, and approvals
- append-only JSON event table for traces
- JSON blob for compiled AgentGraph IR snapshots

Suggested tables:

- `graph_snapshots`
- `runtime_observed_state`
- `runs`
- `trace_events`
- `approvals`
- `eval_results`

This keeps v0.1 simple while preserving a path to richer queries.

## Validation Rules

AgentGraph IR validation:

- Node IDs are unique.
- Edge IDs are unique.
- Edge endpoints exist.
- Referenced models, runtimes, tools, policies, and budgets exist.
- No raw secrets are present.
- Trust defaults are applied.
- Unsupported executable edge kinds are flagged.
- Runtime nodes have valid runner definitions.
- External agent nodes have adapter or endpoint bindings.

Adapter contract validation:

- Adapter kind is known.
- Required target fields exist.
- Auth references exist but values are not embedded in public responses.
- Capability manifest is present or defaults to unknown.
- Unsupported operations return `unsupported`, not generic failure.

Runtime reconciler validation:

- Desired runtime phase is valid.
- Local process command is structured.
- Workspace path is inside allowed scope if policy requires it.
- Port conflicts are reported.
- Health check kind is supported.

Trace validation:

- Event type is known or namespaced.
- Event sequence increases within run.
- Required event fields exist.
- Payload is JSON.
- Redaction metadata is present when secret-like fields are removed.

## Implementation Plan

Phase 1:

- Define minimal Go structs for the proof-slice graph snapshot.
- Define Go structs for adapter contract types.
- Define trace event structs.
- Add JSON fixtures for sample graph.
- Add validation tests.

Phase 2:

- Compile minimal AgentSpec into AgentGraph IR.
- Store graph snapshot in SQLite or in-memory placeholder.
- Render graph in Console from IR endpoint.

Phase 3:

- Implement OpenAI-compatible adapter.
- Implement local process runtime observed state.
- Emit trace events for health and invoke.

Phase 4:

- Add approval and policy event integration.
- Add trace export JSONL.
- Add eval hook storage stubs.

## Open Questions

- Should AgentGraph IR be a public API in v0.1 or remain internal?
- Should IR snapshots be stored as full JSON blobs, normalized tables, or both?
- Should adapter streams use Server-Sent Events, WebSocket, or internal Go channels first?
- Should `operator` be a trust level or a separate endpoint classification?
- Should eval hooks run in-process, as local subprocesses, or as adapter-like plugins?
- Should unknown future event types be accepted with a namespace prefix?
