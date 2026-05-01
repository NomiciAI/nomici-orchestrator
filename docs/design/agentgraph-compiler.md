# AgentGraph Compiler Design

## Purpose

The AgentGraph compiler turns user-facing definitions into internal AgentGraph IR.

Inputs:

- AgentSpec
- installed pack graph fragments
- Console edits
- templates

Output:

- validated AgentGraph IR snapshot

AgentGraph IR is internal in v0.1.

## Convergence Policy

This document describes the target internal shape, not a requirement to implement every IR field before the first adapter works.

Implementation should be evidence-driven:

- start with the smallest immutable graph snapshot needed by the proof slice
- implement OpenAI-compatible invocation first
- add fields only when an adapter, pack, policy check, trace view, or Console feature needs them
- keep source mapping early because it drives actionable errors
- keep snapshot immutability early because it drives trace, replay, and audit

Do not expose AgentGraph IR as a public standard in v0.1.

## Compile Pipeline

```text
load sources
  -> parse
  -> normalize
  -> resolve references
  -> apply defaults
  -> attach capabilities
  -> attach trust profiles
  -> attach policies
  -> validate
  -> snapshot
```

## Source Mapping

Each IR node and edge should preserve source metadata:

```json
{
  "source": {
    "kind": "agentspec",
    "file": "nomici.yaml",
    "path": "agents.product_pm"
  }
}
```

This enables useful validation errors:

```text
agents.product_pm.model references missing model "gpt"
```

## Validation Passes

Required passes:

- unique node IDs
- unique edge IDs
- valid node kinds
- valid edge kinds
- edge endpoints exist
- model references exist
- runtime references exist
- tool references exist
- policy references exist
- no raw secrets
- trust defaults applied
- unsupported executable edges flagged

The first implementation may support a reduced pass set if unsupported features fail closed with clear errors.

## Edge Semantics

Internal graph coordination:

- `delegates_to`
- `handoff`
- `agent_as_tool`
- `parallel`
- `fan_in`
- `fallback`

Protocol/data access:

- `a2a`
- `uses_tool`
- `uses_mcp`
- `uses_model`
- `reads_memory`
- `writes_memory`

Governance:

- `requires_approval`
- `deploys_to`

A2A should be used when crossing runtime, server, vendor, or remote-agent boundaries.

Memory edge semantics:

- `reads_memory` may mean reading Nomici Shared Context or declaring that a runtime has its own memory.
- `writes_memory` may mean promoting a trace/artifact summary into Shared Context.
- These edges do not imply Nomici owns agent-native memory.
- v0.1 should preserve and render memory edges even when only handoff context snapshots are executable.

## Graph Snapshot

Snapshot:

```json
{
  "snapshot_id": "graph_01H",
  "schema_version": "0.1",
  "project_id": "ai-application-pm",
  "created_at": "2026-05-01T00:00:00Z",
  "source_hash": "sha256:...",
  "ir": {}
}
```

Snapshots are immutable. New applies create new snapshots.

## CLI

```bash
nomici graph validate
nomici graph render
nomici graph export --format json
nomici graph export --format yaml
```

## Gateway API

```text
POST /api/graphs/compile
GET  /api/graphs/current
GET  /api/graphs/snapshots
GET  /api/graphs/snapshots/{id}
POST /api/graphs/apply
```

## Error Shape

```json
{
  "code": "missing_reference",
  "message": "Agent product_pm references missing model gpt.",
  "source": {
    "file": "nomici.yaml",
    "path": "agents.product_pm.model"
  },
  "remediation": "Define models.gpt or update the agent model reference."
}
```

## Tests

- compile minimal AgentSpec
- compile pack fragment
- missing reference errors
- duplicate ID errors
- secret detection
- unsupported edge behavior
- snapshot hash stability
