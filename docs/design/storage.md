# Storage Design

## Purpose

Nomici needs local-first persistent state for profiles, provider profiles, graph snapshots, runtime observations, runs, traces, approvals, artifacts, and eval results.

SQLite is the default v0.1 store.

## State Locations

Workspace:

```text
project/
  nomici.yaml
  .nomici/
    state.db
    runs/
    artifacts/
    logs/
```

Global:

```text
~/.nomici/
  config.yaml
  profiles/
  secrets/
  cache/
```

Rules:

- `nomici.yaml` is Git-friendly.
- `.nomici/` is ignored.
- raw secrets are not stored in AgentSpec.
- global provider profiles can be referenced by project config.

## SQLite Tables

Initial tables:

```text
profiles
provider_profiles
pack_installations
graph_snapshots
runtime_desired_state
runtime_observed_state
runs
tasks
trace_events
approvals
artifacts
eval_results
migrations
```

## Table Sketches

`graph_snapshots`:

```text
id
schema_version
project_id
source_hash
ir_json
created_at
```

`runtime_observed_state`:

```text
runtime_id
phase
pid
endpoint
health_json
restart_count
last_error
updated_at
```

`trace_events`:

```text
event_id
run_id
sequence
type
time
node_id
runtime_id
payload_json
redactions_json
metadata_json
```

`approvals`:

```text
approval_id
run_id
status
risk
summary
payload_json
decision_json
created_at
decided_at
```

`artifacts`:

```text
artifact_id
run_id
kind
path
metadata_json
created_at
```

## Migrations

v0.1 should include a simple migration runner:

- migrations are numbered
- applied migrations are recorded
- migration failures stop Gateway startup
- destructive migrations require explicit release notes

## Retention

Initial behavior:

- keep traces indefinitely
- manual cleanup only

Later:

- retention by age
- retention by size
- archive/export

## Redaction

Storage may contain sensitive data. Export paths must redact:

- API keys
- bearer tokens
- Authorization headers
- private SSH keys
- raw env values

Debug bundles must be reviewed and redacted.

## Postgres Path

Postgres is deferred until team/server mode.

Design requirements:

- avoid SQLite-only assumptions in service interfaces
- keep SQL simple
- use explicit transactions
- keep event payloads JSON-compatible

## Tests

- migration apply
- migration idempotency
- trace insert ordering
- JSONL export redaction
- workspace/global path separation
- corrupt DB startup error
