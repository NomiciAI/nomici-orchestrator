# Design Deep Dives

These documents are implementation-oriented design notes.

RFCs capture decisions and direction. Design deep dives turn the direction into data models, state machines, APIs, safety boundaries, and implementation phases.

Current deep dives:

- [System Composition](system-composition.md)
- [v0.1 Boundaries](v0.1-boundaries.md)
- [Shared Context and Autonomy](shared-context-autonomy.md)
- [CLI Agent Runtimes](cli-agent-runtimes.md)
- [Provider Setup](provider-setup.md)
- [Pack System](packs.md)
- [AgentGraph Compiler](agentgraph-compiler.md)
- [Gateway API](gateway-api.md)
- [Runtime Reconciler](runtime-reconciler.md)
- [Run, Task, Trace](run-task-trace.md)
- [Policy and Tool Broker](policy-tool-broker.md)
- [Storage](storage.md)

## Cross-System Flow

The intended v0.1 vertical slice:

```text
Provider Setup
  -> Model Profile
  -> Pack Install
  -> AgentSpec Fragment
  -> AgentGraph Compile
  -> Runtime Reconcile
  -> Run Engine
  -> Policy / Approval / Tool Broker
  -> Trace Events
  -> Artifacts
```

Design rules:

- Gateway owns coordination.
- CLI and Console call Gateway APIs once Gateway is running.
- AgentGraph IR is internal.
- Provider profiles and pack manifests are user-facing extension contracts.
- Side effects go through Policy, Approval, and Trace.
- SQLite is the v0.1 source of observed state.

The composition rationale is documented in [System Composition](system-composition.md).

Scope and naming clarifications are documented in [v0.1 Boundaries](v0.1-boundaries.md).

Agent-native memory, shared context bridging, and long-running autonomy are covered in [Shared Context and Autonomy](shared-context-autonomy.md).

Claude Code, Codex, opencode, Aider, custom commands, and editor-native agents with automation surfaces are covered in [CLI Agent Runtimes](cli-agent-runtimes.md).
