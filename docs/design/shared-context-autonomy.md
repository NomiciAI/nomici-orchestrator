# Shared Context and Autonomy

## Purpose

Nomici should support long-running multi-agent work without weakening the intelligence of the runtimes underneath it.

Hermes, OpenClaw, Claude Code, Codex, opencode, Aider, LangGraph, OpenAI Agents SDK, and other runtimes may have their own memory, sessions, planning, tool use, and learning loops. Nomici should not replace those systems.

Nomici needs a different layer:

```text
Shared Context Layer
  cross-agent context
  handoff briefings
  project decisions
  run summaries
  open issues
  user feedback
  approval lessons
```

This is control-plane context, not agent-native memory.

## Memory Taxonomy

Agent-native memory:

- owned by the runtime
- used inside that agent's reasoning loop
- may include user preferences, project facts, session history, skills, and private runtime state
- examples: Hermes memory, OpenClaw memory, framework sessions, local coding-agent session state

Nomici Shared Context:

- owned by Nomici Gateway
- used at invocation and handoff boundaries
- stores structured context that helps agents coordinate
- does not replace runtime memory
- does not require raw memory dumps from runtimes

Trace and artifacts:

- evidence of what happened
- append-only or artifact-backed
- used for audit, replay, eval, debugging, and user review
- not automatically injected into future runs unless promoted into shared context

Rules:

- Do not read or overwrite agent-native memory unless an adapter explicitly supports safe export/import.
- Do not ask runtimes to dump private memory by default.
- Use summaries, decisions, artifacts, and handoff briefings as the bridge.
- Keep raw prompts, secrets, and private files out of shared context unless explicitly allowed.

## Context Scopes

Project Context:

- project facts
- coding standards
- architecture decisions
- repo layout
- user preferences for this project
- known constraints

Run Context:

- current objective
- task decomposition
- agent assignments
- artifacts produced
- open issues
- known risks
- decisions made during the run

Handoff Context:

- what the upstream agent did
- why it made key decisions
- what it tried and rejected
- what remains unresolved
- what the downstream agent should focus on
- artifact and diff references

Cross-Session Context:

- prior similar runs
- recurring failures
- successful agent pairings
- user feedback on outputs
- approval patterns within the same policy scope

## Data Model

Context item:

```json
{
  "context_id": "ctx_01H",
  "scope": "project",
  "kind": "decision",
  "title": "Use JWT for stateless auth",
  "body": "JWT was selected because the app needs stateless API auth and does not yet require server-side session invalidation.",
  "source": {
    "kind": "run",
    "run_id": "run_01H",
    "agent_id": "architect"
  },
  "confidence": "human_confirmed",
  "sensitivity": "normal",
  "status": "active",
  "created_at": "2026-05-01T00:00:00Z"
}
```

Context snapshot:

```json
{
  "snapshot_id": "ctxsnap_01H",
  "run_id": "run_01H",
  "from_agent": "implementer",
  "to_agent": "reviewer",
  "summary": "Implemented login middleware, added tests, left refresh-token rotation unresolved.",
  "artifacts": ["artifact_diff_01H"],
  "open_issues": [
    "Refresh-token rotation is not implemented."
  ],
  "recommendations": [
    "Review auth middleware error handling."
  ]
}
```

Recommended context kinds:

- `fact`
- `decision`
- `preference`
- `constraint`
- `open_issue`
- `lesson`
- `handoff_briefing`
- `approval_pattern`
- `artifact_summary`
- `eval_feedback`

Confidence values:

- `observed`
- `agent_suggested`
- `human_confirmed`
- `stale`
- `conflicting`

Sensitivity values:

- `public`
- `normal`
- `sensitive`
- `secret_ref`

Shared context must not store raw secrets.

## Adapter Boundary

Invoke requests should carry a task briefing:

```json
{
  "shared_context": {
    "project": [],
    "run": [],
    "handoff": {
      "snapshot_id": "ctxsnap_01H",
      "summary": "..."
    }
  }
}
```

Invoke results may return a context snapshot:

```json
{
  "context_snapshot": {
    "summary": "Completed implementation and found one unresolved edge case.",
    "open_issues": [],
    "decisions": [],
    "artifacts": []
  }
}
```

The adapter does not need to expose internal memory. It only needs to accept briefings and optionally return summaries.

## AgentGraph Semantics

`memory` nodes and `reads_memory` / `writes_memory` edges should not imply that Nomici owns all agent memory.

They mean one of these:

- this agent has an external runtime memory system
- this graph uses Nomici Shared Context
- this edge needs context briefing
- this step may promote trace/artifact information into shared context

v0.1 should treat memory graph elements as declarations and visualization aids unless the Shared Context service supports the required operation.

## Autonomy Model

Nomici's policy layer should not make agents less useful. It should make long-running autonomy safe enough to trust.

Default autonomy tiers:

Low risk:

- read files inside workspace
- inspect repo state
- run tests
- produce plans
- generate diffs
- summarize artifacts

Default: allow and trace.

Medium risk:

- write files inside approved workspace
- run known local commands
- call known APIs
- create local branches
- use trusted MCP tools

Default: allow within scoped policy, or require one approval per run/session.

High risk:

- git push
- PR creation
- deploy
- writing outside workspace
- destructive file operations
- unknown network hosts
- sending email or calendar mutations
- invoking untrusted operator tools

Default: require explicit approval.

Critical risk:

- raw secret export
- permission bypass flags
- public remote access enablement
- destructive system paths
- force push without explicit policy

Default: deny unless explicitly configured.

## Long-Running Work

Nomici should make this workflow possible:

```text
root agent
  -> task ledger decomposition
  -> architect
  -> implementer
  -> reviewer
  -> test runner
  -> implementer retry loop if tests fail
  -> human approval gate before publish/deploy
```

Control-plane responsibilities:

- task ledger
- assignment
- handoff context snapshots
- artifacts and diffs
- retry/fallback routing
- approval gates
- trace timeline
- eval feedback

Data-plane responsibilities:

- agent reasoning
- agent-native memory
- tool execution inside runtime boundaries
- durable workflow execution where a specialized runtime provides it

Nomici should not claim full autonomy in v0.1. It should provide safe partial autonomy with checkpoints.

## v0.1 Scope

Implement:

- project context items
- run context items
- handoff context snapshots
- adapter request briefing field
- adapter result context snapshot field
- trace events for context creation and handoff attachment
- simple Console/CLI inspection later in the product slice

Defer:

- vector memory
- autonomous memory compaction
- cross-project memory
- automatic preference learning without user confirmation
- direct import/export of Hermes/OpenClaw private memory
- replacing runtime sessions or memory

## Tests

- context item validation
- secret redaction
- handoff snapshot creation
- adapter request includes allowed briefing only
- adapter result promotes allowed context only
- stale/conflicting context handling
- policy tests for low/medium/high/critical autonomy tiers
- trace export redacts sensitive context
