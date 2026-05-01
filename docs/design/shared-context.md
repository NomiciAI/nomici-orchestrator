# Shared Context Design

## Purpose

Shared Context is Nomici's control-plane context layer for multi-agent work.

It solves a problem that individual agent runtimes do not solve: when work moves from one agent to another, the downstream agent needs useful task context without requiring access to the upstream runtime's private memory, session state, prompts, or tools.

Shared Context is not agent-native memory. Hermes, OpenClaw, Claude Code, Codex, opencode, Aider, LangGraph, CrewAI, OpenAI Agents SDK, and other runtimes keep ownership of their own memory and execution state.

Nomici owns:

- project decisions
- run summaries
- handoff briefings
- open issues
- artifact summaries
- user feedback
- approval patterns within explicit policy scope

## Core Types

Shared Context has two v0.1 records:

- `context_item`: persistent or run-scoped context that can be selected for future briefings.
- `context_snapshot`: point-in-time handoff briefing created for a specific upstream/downstream transition.

### Context Item

```json
{
  "context_id": "ctx_01H",
  "project_id": "proj_01H",
  "scope": "project",
  "kind": "decision",
  "title": "Use JWT for stateless API auth",
  "body": "JWT was selected because the app needs stateless API auth and does not yet require server-side session invalidation.",
  "tags": ["auth", "backend"],
  "subject_refs": [
    {
      "kind": "path",
      "value": "server/auth"
    }
  ],
  "source": {
    "kind": "run",
    "run_id": "run_01H",
    "task_id": "task_01H",
    "agent_id": "architect",
    "event_id": "evt_01H"
  },
  "confidence": "human_confirmed",
  "sensitivity": "normal",
  "status": "active",
  "ttl": null,
  "created_at": "2026-05-01T00:00:00Z",
  "updated_at": "2026-05-01T00:00:00Z"
}
```

Required fields:

- `context_id`
- `project_id`
- `scope`
- `kind`
- `title`
- `body`
- `source`
- `confidence`
- `sensitivity`
- `status`
- `created_at`

Recommended optional fields:

- `run_id`
- `task_id`
- `agent_id`
- `agent_pair`
- `task_type`
- `tags`
- `subject_refs`
- `artifact_refs`
- `expires_at`
- `supersedes`
- `metadata`

### Context Snapshot

```json
{
  "snapshot_id": "ctxsnap_01H",
  "project_id": "proj_01H",
  "run_id": "run_01H",
  "task_id": "task_01H",
  "from_agent": "implementer",
  "to_agent": "reviewer",
  "summary": "Implemented login middleware, added tests, and left refresh-token rotation unresolved.",
  "decisions": [
    {
      "title": "Keep refresh-token rotation out of this diff",
      "body": "The first pass focuses on request auth middleware; token rotation needs a separate schema change."
    }
  ],
  "open_issues": [
    "Refresh-token rotation is not implemented."
  ],
  "recommendations": [
    "Review auth middleware error handling and test coverage."
  ],
  "artifact_refs": ["artifact_diff_01H"],
  "context_item_refs": ["ctx_01H"],
  "created_by": {
    "kind": "adapter_result",
    "agent_id": "implementer"
  },
  "created_at": "2026-05-01T00:00:00Z"
}
```

Snapshots are immutable once attached to a handoff. If a correction is needed, create a new snapshot and link it through `supersedes`.

## Context Kinds

v0.1 should use a small controlled vocabulary plus open-ended tags.

Controlled `kind` values:

- `fact`
- `decision`
- `preference`
- `constraint`
- `run_summary`
- `task_summary`
- `open_issue`
- `lesson`
- `handoff_briefing`
- `artifact_summary`
- `eval_feedback`
- `approval_pattern`

Rules:

- `kind` drives selection, rendering, and default retention.
- `tags` are open-ended and user or pack defined.
- Unknown `kind` values should fail validation unless carried under an explicit extension namespace.
- Unknown `tags` are allowed.

This keeps the core model predictable without preventing pack-specific context labels.

## Scopes and Lifecycle

Scopes:

- `project`: durable project knowledge. Keep until removed, superseded, or expired.
- `run`: valid only for one run. Keep with trace by default, but do not inject into future runs unless promoted.
- `task`: attached to a task ledger item. Keep with the run.
- `handoff`: attached to one handoff edge. Immutable once used.
- `agent_pair`: reusable lesson about a pair of agents inside one project.
- `policy`: approval pattern or safety note inside explicit policy scope.

Status values:

- `active`
- `superseded`
- `stale`
- `conflicting`
- `rejected`

Retention defaults:

- Project context: keep indefinitely.
- Run, task, and handoff context: keep with run trace.
- Agent-pair context: keep indefinitely, but mark stale after version/provider/runtime changes.
- Policy context: keep only inside its approval scope and never promote silently to global policy.

Cross-run association keys:

- `project_id`
- `task_type`
- `agent_id`
- `agent_pair`
- `subject_refs`
- `tags`
- `artifact_refs`

v0.1 should avoid semantic vector retrieval. Selection can start with explicit references, tags, subject refs, and recent project decisions.

## Snapshot Creation

Private bootstrap implementation note:

- The first executable path generates snapshots for `cli_agent` runs.
- If a CLI agent prints structured JSON with a top-level `context_snapshot`, Nomici stores that candidate after validation and redaction.
- Otherwise Nomici falls back to a deterministic snapshot from stdout, errors, changed files, and artifact refs.
- The first supported handoff path is one `handoff` edge between two `cli_agent`-backed `external_agent` nodes.
- Snapshot promotion into durable project context is still deferred.

Nomici should support two snapshot creation paths.

Adapter-provided snapshot:

```text
adapter invoke result
  -> context_snapshot candidate
  -> Gateway validation/redaction
  -> trace event
  -> attach to handoff
```

Gateway-generated snapshot:

```text
trace events + artifacts + task state
  -> deterministic summarizer or rule-based summary
  -> context_snapshot candidate
  -> Gateway validation/redaction
  -> trace event
  -> attach to handoff
```

v0.1 priority:

1. Accept adapter-provided `context_snapshot` when available.
2. Fall back to a simple Gateway-generated snapshot from output summary, artifacts, open issues, and task state.
3. Do not infer durable project context automatically unless policy allows or a human confirms it.

This avoids blocking on automatic memory extraction while still giving every handoff a structured briefing.

## Adapter Contract

Invoke requests carry Shared Context as a structured field.

```json
{
  "input": {
    "messages": []
  },
  "shared_context": {
    "project": [
      {
        "context_id": "ctx_01H",
        "kind": "decision",
        "title": "Use JWT for stateless API auth",
        "body": "JWT was selected because the app needs stateless API auth."
      }
    ],
    "run": [],
    "handoff": {
      "snapshot_id": "ctxsnap_01H",
      "summary": "Implementer created auth middleware and left token rotation unresolved.",
      "open_issues": [
        "Refresh-token rotation is not implemented."
      ],
      "artifact_refs": ["artifact_diff_01H"]
    }
  }
}
```

Invoke results may return a candidate snapshot.

```json
{
  "status": "completed",
  "output": {
    "messages": []
  },
  "context_snapshot": {
    "summary": "Completed scaffold implementation and found one follow-up test gap.",
    "decisions": [],
    "open_issues": [
      "Add integration test for auth failure path."
    ],
    "recommendations": [
      "Ask reviewer to focus on middleware error handling."
    ],
    "artifact_refs": ["artifact_diff_01H"]
  }
}
```

Rules:

- Gateway decides which context items are allowed into the request.
- Adapters may ignore unknown context fields.
- Adapters must not return raw secrets or private runtime memory dumps as context.
- Gateway validates, redacts, and stores accepted snapshots.
- A context snapshot is not automatically promoted to project context.

## Prompt Injection Boundary

Gateway stores context structurally. Each adapter chooses how to present it to the underlying runtime.

For model or HTTP adapters:

- pass `shared_context` as a structured field where the runtime supports it
- otherwise render it into a bounded task briefing block

For CLI agent runners:

- render a task briefing from the selected context
- inject it through the configured prompt template or stdin
- avoid mixing secrets, raw trace payloads, or hidden policy state into the prompt

Recommended briefing shape:

```text
Task briefing:
- Objective: ...
- Project decisions: ...
- Upstream handoff: ...
- Artifacts to inspect: ...
- Open issues: ...
- Constraints: ...
```

The briefing is part of the adapter input and should be traced with redaction metadata.

## Promotion Rules

Context can be promoted from trace, artifact, or snapshot into durable project context.

Promotion sources:

- human-confirmed item
- pack-authored item
- adapter-proposed item with policy approval
- eval feedback accepted by user

v0.1 should require explicit human confirmation for:

- user preferences
- durable architecture decisions
- approval patterns that reduce future friction
- any context marked `sensitive`

Automatic promotion is deferred except for safe, pack-authored bootstrap context.

## Storage

SQLite tables should support the lifecycle directly.

`context_items`:

```text
context_id
project_id
run_id
task_id
agent_id
agent_pair
task_type
scope
kind
title
body
tags_json
subject_refs_json
artifact_refs_json
source_json
confidence
sensitivity
status
expires_at
supersedes
metadata_json
created_at
updated_at
```

`context_snapshots`:

```text
snapshot_id
project_id
run_id
task_id
from_agent
to_agent
summary
decisions_json
open_issues_json
recommendations_json
artifact_refs_json
context_item_refs_json
created_by_json
supersedes
created_at
```

## API Sketch

v0.1 API group:

```text
GET    /api/context/items
POST   /api/context/items
GET    /api/context/items/{id}
PATCH  /api/context/items/{id}
POST   /api/context/items/{id}/promote
POST   /api/context/snapshots
GET    /api/context/snapshots/{id}
GET    /api/runs/{run_id}/context
```

Every mutation should emit trace events:

- `context.item.created`
- `context.item.updated`
- `context.item.promoted`
- `context.snapshot.created`
- `handoff.context_attached`

## Security

Shared Context can leak sensitive project information if treated casually.

Rules:

- never store raw secrets
- store secret references only as `secret_ref`
- redact raw prompts, env values, bearer tokens, private keys, and credentials
- mark context sensitivity before injection
- do not inject sensitive context into untrusted external agents unless policy allows it
- include context in debug bundle redaction
- include context exports in secret scans

Sensitivity values:

- `public`
- `normal`
- `sensitive`
- `secret_ref`

## v0.1 Non-Goals

Do not implement in v0.1:

- vector memory
- autonomous memory compaction
- cross-project memory
- automatic preference learning without confirmation
- direct import/export of Hermes/OpenClaw private memory
- raw runtime memory dumps
- global learned policy exceptions

## Tests

Required tests:

- context item validation
- context snapshot validation
- redaction before storage and export
- handoff snapshot attachment
- adapter request contains only allowed context
- adapter result snapshot promotion requires validation
- stale and superseded item handling
- run context is not injected into future runs unless promoted
- sensitive context is blocked for untrusted agents by default
