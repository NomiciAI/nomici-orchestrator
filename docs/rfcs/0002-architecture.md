# RFC 0002: Architecture

Status: Draft
Date: 2026-04-30
Target release: Nomici Orchestrator v0.1

## Summary

Nomici Orchestrator uses a Gateway-centered architecture.

The CLI, Web Console, OpenAI-compatible endpoint, A2A endpoint, MCP registry, runtime manager, policy engine, approval queue, and trace store should all converge through Nomici Gateway. The Gateway owns the control plane. External agents, local model servers, MCP servers, and worker processes form the data plane.

The recommended v0.1 implementation stack is:

- Go for CLI and Gateway
- React, Vite, React Flow, and Tailwind for Nomici Console
- Embedded static Web Console assets served by the Gateway
- SQLite with WAL for default local state
- JSON Schema for AgentSpec validation
- TypeScript package for Console types and future JS SDK
- Python package later for adapter authoring and examples

## Core Architecture

```text
┌───────────────────────────────────────────────────────────────┐
│                        Nomici Console                         │
│  Dashboard / Canvas / Agents / Runtimes / Models / Runs       │
│  Tools / Approvals / Policy / Settings                        │
└───────────────────────────────▲───────────────────────────────┘
                                │
                                │ REST / WebSocket
                                │
┌───────────────────────────────┴───────────────────────────────┐
│                        Nomici Gateway                          │
│                                                               │
│  Spec Loader        Agent Registry      Runtime Manager        │
│  Model Registry     Tool Registry       Adapter Layer          │
│  Run Engine         Policy Engine       Approval Queue         │
│  Trace Store        Secrets Manager     Event Stream           │
│  OpenAI-compatible API                  Health API             │
└───────────────▲───────────────▲──────────────▲────────────────┘
                │               │              │
                │               │              │
         A2A / HTTP       MCP / JSON-RPC       Process / Docker
                │               │              │
┌───────────────┴──────┐ ┌──────┴───────────┐ ┌┴────────────────┐
│  External Agents     │ │  Tools / Data     │ │ Local Runtimes   │
│  CLI agents          │ │  Filesystem MCP   │ │ Ollama           │
│  Claude/Codex/etc.   │ │  GitHub MCP       │ │ vLLM             │
│  Hermes / OpenClaw   │ │  Browser MCP      │ │ SGLang           │
│  LangGraph / CrewAI  │ │  Slack MCP        │ │ local processes  │
│  ADK                 │ │  Postgres MCP     │ │ Docker workers   │
└──────────────────────┘ └──────────────────┘ └─────────────────┘
```

## Control Plane and Data Plane

Control plane:

- Agent registry
- Runtime lifecycle
- Model registry
- Tool and MCP registry
- Graph definition
- Policies
- Approval queue
- Trace and audit log
- Budget metadata
- Secrets references
- Deployment state
- Workspace and profile state

Data plane:

- CLI-capable agents such as Claude Code, Codex, opencode, Aider, custom commands, and editor-native agents where they expose automation surfaces
- Hermes runtime
- OpenClaw Gateway
- LangGraph worker
- OpenAI Agents SDK worker
- CrewAI process
- Ollama or vLLM local model server
- MCP server
- Browser, file, shell, and API tools

Nomici should avoid taking ownership of data-plane internals unless a native adapter is intentionally implemented.

## Process Model

Nomici has two primary process modes:

```bash
nomici gateway run
```

Runs the Gateway in the foreground. This is the preferred development, debug, WSL, and tmux mode.

```bash
nomici gateway start
```

Starts the Gateway as a background daemon or service where supported.

`nomici up` should:

1. Load the selected profile and workspace.
2. Read `nomici.yaml`.
3. Validate AgentSpec.
4. Start or connect to Nomici Gateway.
5. Apply the desired state to the Gateway.
6. Start configured local runtimes.
7. Run health checks.
8. Print the Console URL and useful next commands.

## CLI and Gateway Relationship

The CLI has two modes:

- Bootstrap mode: commands needed before Gateway is available, such as `init`, `spec validate`, and `gateway run`.
- Client mode: commands that call the Gateway API, such as `runtime start`, `agent run`, `trace show`, and `approvals grant`.

Once Gateway is running, CLI behavior should match Web Console behavior because both should use the same API and policy path.

## Gateway API Surfaces

Core REST API:

- `/api/health`
- `/api/agents`
- `/api/models`
- `/api/runtimes`
- `/api/tools`
- `/api/graphs`
- `/api/runs`
- `/api/context`
- `/api/traces`
- `/api/approvals`
- `/api/policies`
- `/api/secrets`

`/api/health` is a health-probe exception and may return a minimal naked JSON response. Normal command/query APIs should use the standard Gateway response envelope.

Event surfaces:

- `/events` for Server-Sent Events if useful for simple clients
- `/ws` for Web Console and richer bidirectional control

OpenAI-compatible API:

- `/v1/models`
- `/v1/chat/completions`
- `/v1/responses`
- `/v1/embeddings`

Protocol surfaces:

- `/a2a/*` for future A2A server and client integration
- `/mcp/*` for future MCP proxy and registry operations

v0.1 should implement only the surfaces required by the MVP and keep future paths reserved but not overpromise full compatibility.

## Internal Components

### Spec Loader

Responsibilities:

- Load `nomici.yaml`.
- Expand environment variable references only where allowed.
- Validate against AgentSpec JSON Schema.
- Produce a normalized desired-state graph.
- Return human-readable validation errors.

The Spec Loader must not resolve secret values into the in-memory public graph returned to the Web Console.

### Registry Layer

Registries store normalized definitions:

- Models
- Runtimes
- Agents
- Tools
- MCP servers
- Edges
- Policies

The registry should distinguish desired state from observed state. Desired state comes from AgentSpec. Observed state comes from runtime health checks, process tables, adapter responses, and trace events.

### Runtime Manager

v0.1 runtime manager supports:

- Local process runner
- PID tracking
- Port allocation or collision detection
- Health checks
- stdout and stderr log capture
- Restart policy metadata
- Workspace path management
- Environment and secret reference injection
- Runtime capability manifest

Docker runner can be designed in the interface but does not need to be production-grade in v0.1 unless implementation cost stays low.

### Adapter Layer

v0.1 adapters:

- OpenAI-compatible endpoint adapter
- generic CLI Agent Runner
- CLI agent profiles for Codex, Claude Code, opencode, Aider, custom commands, and editor-native agents where they expose automation surfaces
- Hermes endpoint adapter
- OpenClaw endpoint adapter
- Ollama provider adapter if direct support is needed

Minimum adapter contract:

- `health`
- `capabilities`
- `invoke`
- `stream`
- `cancel`
- `logs` when applicable

Later adapter contract:

- `start`
- `stop`
- `inspect`
- `listSessions`
- `exportTrace`
- `approvalBridge`

### Run Engine

The Run Engine coordinates a run without replacing specialized runtimes.

v0.1 run capabilities:

- Invoke one agent.
- Route to an external endpoint.
- Emit run lifecycle events.
- Stream model or agent output when supported.
- Ask the policy engine before risky Nomici-mediated actions.
- Create approval records.
- Cancel a run where the adapter supports cancellation.

Deferred orchestration features:

- Durable workflow engine
- Complex fan-out and fan-in
- Native A2A task orchestration
- Deep framework-specific state resume

### Policy Engine

The policy engine evaluates actions before execution.

Inputs:

- Actor
- Agent
- Runtime
- Tool or operation
- Arguments metadata
- Workspace
- Trust level
- Policy config

Outputs:

- allow
- deny
- require approval
- require stronger auth

### Trace Store

Default storage:

- SQLite
- WAL enabled
- Append-only events table
- Derived run summary tables for UI performance

Trace replay in v0.1 means timeline replay, not deterministic re-execution.

## State Layout

Global state:

```text
~/.nomici/
  config.yaml
  profiles/
  secrets/
  logs/
  traces/
  runtimes/
  cache/
```

Workspace state:

```text
project/
  nomici.yaml
  .nomici/
    state.db
    runs/
    artifacts/
    workspaces/
```

Rules:

- `nomici.yaml` is intended to be committed.
- `.nomici/` is local state and should be ignored by default.
- Secrets must not be written to `nomici.yaml`.
- Secret references may be stored in AgentSpec.

## Build and Release Shape

Recommended repository layout:

```text
orchestrator/
  cmd/
    nomici/
  internal/
    gateway/
    runtime/
    registry/
    policy/
    trace/
    secrets/
    spec/
  apps/
    web/
  packages/
    spec/
    client-js/
    client-python/
  adapters/
    hermes/
    openclaw/
  examples/
  scripts/
  docs/
```

The Gateway binary should embed the built Web Console. Development may run Vite separately, but release users should not need Node.js to open the Console.

## Implementation Phases

Phase 1:

- Repo skeleton
- Go CLI scaffold
- Gateway health endpoint
- Embedded static asset placeholder
- SQLite state initialization
- AgentSpec parser and schema validation

Phase 2:

- Runtime registry
- Local process runner
- Logs and health checks
- `nomici up`, `ps`, `logs`, `down`

Phase 3:

- Model and agent registry
- OpenAI-compatible adapter
- generic CLI Agent Runner
- Basic `agent run`
- Event log and trace export

Phase 4:

- React Console dashboard and canvas
- WebSocket event stream
- Approvals UI
- YAML import and export

Phase 5:

- Hermes and OpenClaw templates
- Demo templates
- Install script
- Security docs and debug bundle

## Open Questions

- Should Gateway state be global per profile or per workspace by default?
- Should `nomici up` apply desired state by replacing the current graph, or patching it?
- Should OpenAI-compatible `/v1/*` route model names to agents, raw models, or both through explicit prefixes?
- Should the Web Console connect only same-origin in v0.1, or support remote Gateway URLs from the start?
- Should Docker runner be in v0.1 or behind an experimental flag?
