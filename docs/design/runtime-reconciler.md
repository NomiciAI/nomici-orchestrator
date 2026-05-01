# Runtime Reconciler Design

## Purpose

The Runtime Reconciler compares desired runtime state with observed runtime state and takes safe actions.

It makes Nomici a control plane rather than a process launcher.

## Desired Runtime State

```json
{
  "runtime_id": "hermes_coder",
  "desired_phase": "running",
  "runner": "local_process",
  "workspace": "./workspaces/hermes-coder",
  "start_command": ["hermes", "-p", "coder", "gateway", "run"],
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

## Observed Runtime State

```json
{
  "runtime_id": "hermes_coder",
  "observed_phase": "running",
  "pid": 12345,
  "endpoint": "http://127.0.0.1:8642/v1",
  "health": {
    "status": "healthy",
    "checked_at": "2026-05-01T00:00:00Z"
  },
  "started_at": "2026-05-01T00:00:00Z",
  "restart_count": 0,
  "last_error": null
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

## Reconcile Actions

- `none`
- `start`
- `stop`
- `restart`
- `health_check`
- `mark_degraded`
- `mark_failed`
- `block`

Safe automatic actions in v0.1:

- health check
- mark degraded
- mark failed
- start during explicit `nomici up`
- stop during explicit `nomici down`

Automatic restart requires explicit restart policy.

## Local Process Runner

Responsibilities:

- structured command execution
- working directory
- env injection through secret refs
- PID tracking
- stdout/stderr log capture
- shutdown signal
- exit code recording

Command should be structured as an array internally, even if CLI accepts a shell-like string.

## CLI Invoke Runner

Command-driven agents such as Claude Code, Codex, opencode, Aider, and similar tools should use a `cli_invoke` runner profile when Nomici is invoking a command rather than maintaining a long-lived server process. Editor-native agents such as Cline and Continue need an actual command-driven automation surface before this runner applies.

Responsibilities:

- resolve executable
- probe version and capabilities
- set working directory
- pass prompt/input through a controlled channel
- capture stdout/stderr or structured stream output
- capture exit code
- support cancellation where possible
- record pre/post workspace diff metadata where configured

The runner should not assume full visibility into internal tool calls made by the CLI agent.

Safety rules:

- explicit workspace required
- no permission-bypass flags by default
- sensitive env vars passed only by reference and only when configured
- writes outside allowed workspace require approval or are blocked
- publishing side effects such as push, PR creation, or deploy require policy gates

## Health Checks

Kinds:

- `http`
- `tcp`
- `command`
- `none`

v0.1 should implement `http` and `tcp`.

Health statuses:

- `unknown`
- `healthy`
- `degraded`
- `unhealthy`
- `stopped`

## Port Conflicts

If desired runtime port is occupied:

- detect before start when possible
- do not kill unknown process
- emit `runtime.start_blocked`
- show remediation

## Trace Events

- `runtime.desired`
- `runtime.started`
- `runtime.stopped`
- `runtime.failed`
- `runtime.health_changed`
- `runtime.reconciled`
- `runtime.start_blocked`

## CLI

```bash
nomici up
nomici down
nomici ps
nomici runtime start <runtime>
nomici runtime stop <runtime>
nomici runtime logs <runtime>
nomici runtime inspect <runtime>
```

## Tests

- desired/observed state transitions
- local process start/stop fixture
- port conflict handling
- health check success/failure
- log capture
- no raw secret logging
