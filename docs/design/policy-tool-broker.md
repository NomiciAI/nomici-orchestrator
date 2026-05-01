# Policy and Tool Broker Design

## Purpose

Nomici should mediate side-effecting tool calls through a policy-aware broker.

This protects local files, shell commands, office documents, browser actions, email/calendar mutations, and remote tools.

## Tool Invocation Path

```text
agent
  -> Gateway Tool Broker
  -> Policy Engine
  -> Approval Queue if needed
  -> Tool Pack / MCP Server / Runtime
  -> Artifact Store
  -> Trace Store
```

## Policy Decision

```json
{
  "decision": "approval",
  "reason": "filesystem write requires approval",
  "risk": "high",
  "policy_refs": ["default.filesystem.write"],
  "approval": {
    "scope_options": ["once", "run", "session"]
  }
}
```

Decision values:

- `allow`
- `deny`
- `approval`

Risk values:

- `low`
- `medium`
- `high`
- `critical`

## Autonomy Tiers

Policy should enable long-running autonomy by distinguishing low-risk work from publish/deploy actions.

Low risk:

- read files inside workspace
- inspect repo state
- run tests
- generate local diffs
- summarize artifacts

Default: allow and trace.

Medium risk:

- write files inside approved workspace
- run known local commands
- call known APIs
- use trusted MCP tools

Default: allow within scoped policy or ask once per run/session.

High risk:

- git push
- PR creation
- deploy
- write outside workspace
- unknown network host
- email/calendar mutation

Default: approval.

Critical risk:

- raw secret export
- permission bypass flags
- destructive system paths
- public remote access enablement

Default: deny unless explicitly configured.

## Risk Classification

Default approval:

- shell exec
- filesystem write
- email send
- calendar mutation
- deploy
- git push
- PR create
- browser side effects
- unknown network host
- untrusted MCP tool
- external operator runtime tool

Default deny:

- raw secret export
- writing outside allowed workspace
- public remote access enablement without explicit config
- destructive system paths

## Approval Record

```json
{
  "approval_id": "approval_01H",
  "run_id": "run_01H",
  "tool_call_id": "tool_01H",
  "status": "pending",
  "risk": "high",
  "summary": "Write docs/output/report.docx",
  "diff_ref": "artifact_diff_01H",
  "requested_at": "2026-05-01T00:00:00Z"
}
```

Statuses:

- `pending`
- `granted`
- `denied`
- `expired`

Scopes:

- `once`
- `run`
- `session`

## Tool Broker Request

```json
{
  "run_id": "run_01H",
  "agent_id": "document_analyst",
  "tool_id": "office.docx.write",
  "arguments_summary": {
    "path": "workspace/output/report.docx"
  },
  "arguments_ref": "encrypted_or_local_ref",
  "trust": {
    "level": "untrusted"
  }
}
```

Arguments may contain private data. Store carefully and redact in exports.

## Artifact Handling

Tools that produce files should register artifacts:

```json
{
  "artifact_id": "artifact_01H",
  "run_id": "run_01H",
  "kind": "docx",
  "path": ".nomici/artifacts/run_01H/report.docx",
  "created_by": "office.docx.write",
  "metadata": {}
}
```

Artifacts live in workspace `.nomici/artifacts` by default.

## MCP Boundary

MCP servers are untrusted by default.

The broker should:

- list tools
- classify risk
- enforce scopes
- request approval
- trace invocation
- capture artifacts

v0.1 may stub MCP while keeping broker interfaces ready.

## CLI

```bash
nomici approvals list
nomici approvals grant <approval_id>
nomici approvals deny <approval_id>
nomici policy check
nomici policy test <fixture>
```

## Tests

- policy decisions
- approval state transitions
- filesystem scope checks
- secret redaction
- artifact registration
- untrusted MCP requires approval
