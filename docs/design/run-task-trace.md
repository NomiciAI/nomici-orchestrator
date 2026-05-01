# Run, Task, Trace Design

## Purpose

Nomici needs a lightweight run engine, a task ledger for long-running work, and an event-sourced trace store.

It should not become a full durable workflow engine in v0.1.

## Run State Machine

```text
created
  -> running
  -> waiting_for_approval
  -> running
  -> completed
```

Terminal states:

- `completed`
- `failed`
- `cancelled`

Run fields:

```json
{
  "run_id": "run_01H",
  "graph_snapshot_id": "graph_01H",
  "entrypoint": "product_pm",
  "status": "running",
  "created_at": "2026-05-01T00:00:00Z",
  "completed_at": null
}
```

## Task Ledger

Tasks give long-running work structure.

```json
{
  "task_id": "task_01H",
  "run_id": "run_01H",
  "title": "Implement login flow",
  "status": "in_progress",
  "assigned_agent": "implementer",
  "checkpoint_ref": "chk_01H",
  "artifacts": ["artifact_01H"],
  "approvals": ["approval_01H"]
}
```

Task statuses:

- `planned`
- `in_progress`
- `blocked`
- `waiting_for_approval`
- `completed`
- `failed`
- `cancelled`

## Trace Event Schema

```json
{
  "event_id": "evt_01H",
  "run_id": "run_01H",
  "parent_event_id": null,
  "sequence": 1,
  "time": "2026-05-01T00:00:00Z",
  "type": "run.started",
  "actor": {
    "kind": "user",
    "id": "local"
  },
  "node_id": "product_pm",
  "runtime_id": null,
  "payload": {},
  "redactions": [],
  "metadata": {}
}
```

Rules:

- append-only
- monotonically increasing sequence per run
- JSON payload
- redaction metadata when secrets are removed
- no raw secret export

## Core Events

- `run.started`
- `run.completed`
- `run.failed`
- `run.cancelled`
- `task.created`
- `task.updated`
- `agent.invoked`
- `agent.completed`
- `adapter.invoked`
- `adapter.completed`
- `model.requested`
- `model.completed`
- `tool.requested`
- `tool.completed`
- `policy.approval_required`
- `approval.requested`
- `approval.granted`
- `approval.denied`
- `artifact.created`
- `eval.scored`

## Timeline Replay

v0.1 replay means:

- show ordered event timeline
- show agent/model/tool transitions
- show approvals
- show artifacts
- show errors

It does not mean deterministic re-execution.

## Eval Hooks

Eval hooks consume completed traces and emit eval events.

v0.1:

- store eval events
- allow manual labels
- export traces for external eval

Deferred:

- automated scoring
- regression dashboards
- judge model integration

## CLI

```bash
nomici run <agent> "..."
nomici trace list
nomici trace show <run_id>
nomici trace export <run_id> --format jsonl
nomici task list
nomici task show <task_id>
```

## Tests

- run state transitions
- trace sequence ordering
- JSONL export
- redaction
- cancellation
- approval wait/resume
