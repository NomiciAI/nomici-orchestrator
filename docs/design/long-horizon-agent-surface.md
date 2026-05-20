# Long-Horizon Agent Surface

## Purpose

Nomici should support agent work that lasts minutes to hours without becoming a hidden agent framework.

Projects such as DeerFlow show a useful product shape: sandboxes, memory, tools, skills, subagents, and a message gateway make long tasks feel like one coherent agent workspace. Nomici should learn from that shape, but keep its own boundary:

- Nomici is the local-first control plane.
- Specialized runtimes keep their own reasoning loops, memory, tools, and sessions.
- Gateway owns graph coordination, policy, approvals, traces, artifacts, and cross-agent context.
- External runtimes are integrated through adapters instead of being absorbed into core.

The implementation details that matter are architectural, not cosmetic. DeerFlow's sandbox layer uses a provider lifecycle (`acquire`, `get`, `release`), deterministic thread-scoped sandbox identity, workspace/uploads/outputs mounts, read-only skills mounts, readiness checks, warm-pool reuse, idle cleanup, and startup reconciliation for orphaned containers. Its subagent layer resolves built-in and custom agents through a registry, applies per-agent overrides, loads skills per subagent, filters tools through skill policy, and passes parent thread/sandbox state into child execution. Nomici should translate those mechanics into control-plane records and adapter contracts before trying to mimic all runtime behavior.

## Product Surface

The user-facing surface should converge around a run workspace:

```text
Run
  -> session
  -> sandbox / workspace
  -> agent graph
  -> tools and skills
  -> shared context
  -> subtask ledger
  -> message stream
  -> artifacts and trace
  -> approvals
```

This gives users a single place to see what is happening while allowing each agent runtime to stay specialized.

## Sandbox

Nomici needs sandbox semantics at the control-plane layer, even when the underlying execution is a CLI agent, a local process, a container, or a remote runtime.

Sandbox responsibilities:

- allocate a per-run or per-agent workspace
- map virtual paths to physical paths where adapters support it
- capture inputs, outputs, diffs, stdout, stderr, and generated artifacts
- enforce declared read/write boundaries through policy
- block protected system paths by default
- make cleanup explicit

v0.1 stance:

- `cli_agent` execution already has a workspace, artifact directory, pre/post diff capture, and workspace lock.
- Gateway now records a per-run sandbox allocation with provider, mode, readiness status, workspace root, artifact root, runtime binary, cleanup status, and trace events.
- The next increment should move from records to provider-backed acquire/get/release adapters for local, container, and remote execution.
- Container isolation is valuable, but should be an adapter/provider capability, not a hard dependency.

## Memory And Session

Nomici should distinguish three concepts that are often blurred:

- session: the live conversation/run state for one user task
- memory: reusable facts, preferences, lessons, and decisions
- trace: evidence of what actually happened

Runtime-native memory remains inside Hermes, OpenClaw, LangGraph, Codex, Claude Code, OpenAI Agents SDK, or other runtimes.

Nomici-owned memory is Shared Context:

- project facts
- run summaries
- handoff briefings
- decisions
- open issues
- approval lessons
- artifact summaries
- user feedback

Session responsibilities:

- hold current messages or task state when Gateway owns the loop
- point to active context snapshots and artifacts
- survive refreshes and Gateway restarts where feasible
- expose resume/cancel status

v0.1 stance:

- keep Shared Context as the bridge between agents
- keep trace as audit evidence
- add session identity and run workspace metadata before adding long-lived autonomous loops
- never import raw runtime memory by default

## Tools And Skills

Tools and skills should be visible capabilities, not hidden prompt content.

Tool responsibilities:

- declare side-effect risk
- declare required secrets and network access
- route through policy before mutation
- emit trace events with redacted payloads
- attach artifacts where useful

Skill responsibilities:

- package domain workflows, prompts, examples, and tool preferences
- be installable through packs
- be enableable per agent or per run
- be visible in Console and graph snapshots

v0.1 stance:

- represent tools and skills in AgentSpec and packs even if execution support is partial
- prefer adapter-mediated tools for external runtimes
- route Nomici-owned tools through Policy and Trace before expanding the tool catalog

## Subagents

DeerFlow's subagent shape maps well to Nomici, but the implementation should stay graph-native.

Subagent responsibilities:

- turn a task into one or more bounded assignments
- run each assignment through an agent/runtime adapter
- stream status back into the parent run
- write a context snapshot when an assignment finishes
- support sequential handoff first, then bounded parallelism

v0.1 stance:

- linear `handoff` chains across `cli_agent`-backed `external_agent` nodes are the first subagent execution path
- each hop receives a bounded Shared Context briefing
- branching, parallel, retries, and fallback need a task ledger before they become default execution behavior

## Message Gateway

The message gateway is the product layer that makes long runs usable.

Responsibilities:

- normalize CLI, Console, API, and future IM-channel input into run messages
- stream status, model output, tool events, approvals, artifacts, and handoffs
- support cancellation and resume
- preserve enough session state to reconnect after UI refresh
- avoid leaking raw secrets or oversized artifacts into message payloads

v0.1 stance:

- Console currently polls run trace events
- the next step is a streaming run event endpoint or SSE-compatible gateway route
- message payloads should be derived from trace/artifact/context records instead of becoming a second source of truth

## Long Task Model

Long tasks need explicit state:

```json
{
  "task_id": "task_01H",
  "run_id": "run_01H",
  "parent_task_id": "",
  "assigned_agent": "implementer",
  "status": "in_progress",
  "context_snapshot_id": "ctxsnap_01H",
  "artifact_refs": [],
  "approval_refs": [],
  "started_at": "2026-05-01T00:00:00Z"
}
```

States:

- `queued`
- `running`
- `waiting_for_approval`
- `blocked`
- `completed`
- `failed`
- `cancelled`

This ledger should become the durable structure under handoffs, parallel subtasks, retries, and resume.

## Implementation Phases

Phase 1:

- linear CLI handoff chains
- per-hop Shared Context snapshots
- trace-visible handoff path
- Console run eligibility for handoff chains

Phase 2:

- run session records
- sandbox/workspace records
- run message stream derived from trace
- cancel/resume hooks
- task ledger for sequential subtasks

Phase 3:

- bounded parallel subagents
- skill enablement per agent/run
- tool broker execution for Nomici-owned tools
- richer Console views for sessions, sandboxes, tasks, and artifacts

Phase 4:

- container or remote sandbox providers
- cross-session memory promotion UI
- IM-channel message gateway
- long-running runtime adapters for LangGraph, OpenAI Agents SDK, CrewAI, Hermes, OpenClaw, and similar systems

## Guardrails

- Do not make Gateway a hidden autonomous agent runtime.
- Do not store raw secrets in sessions, memory, trace, or artifacts.
- Do not dump external runtime memory unless an adapter explicitly supports safe export.
- Do not run unbounded parallel agents without workspace locking and budget controls.
- Do not let message streams become the system of record; trace, context, task, and artifact stores remain authoritative.
