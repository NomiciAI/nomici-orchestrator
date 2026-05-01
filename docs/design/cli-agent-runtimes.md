# CLI Agent Runtimes

## Purpose

Claude Code, Codex, opencode, Aider, and similar command-driven local agents are high-value data-plane runtimes for Nomici.

Cline, Continue, and similar editor-native agents still matter to the strategy, but v0.1 should treat them as CLI-agent candidates only when they expose a command-driven automation surface. Otherwise they belong in future sidecar or extension integrations.

They are not competitors to replace in v0.1. They are exactly the kind of strong local runtime that makes an independent control plane useful.

The important abstraction is not a Claude Code adapter, a Codex adapter, or an opencode adapter. It is a generic CLI Agent Runner.

## Strategic Difference

Local coding agents solve:

```text
one strong agent working inside one project
```

Nomici should solve:

```text
many heterogeneous agents, runtimes, tools, models, policies, approvals,
traces, and artifacts coordinated as one agent organization
```

The difference is not that Nomici writes code better than Claude Code, Codex, opencode, Aider, Cline, or Continue.

The difference is that Nomici provides:

- runtime-neutral orchestration
- multi-agent graph composition
- model/provider mixing
- policy and approval gates
- cross-runtime trace and audit
- pack-based team templates
- visible handoff and review flows
- local-first governance around powerful tools

## Why They Strengthen Nomici

Strong local CLI agents create the need for a control plane.

A realistic user may want:

```text
product_pm        gateway_agent using configured provider
architect         gateway_agent using local Ollama
implementer       external_agent backed by Claude Code
reviewer          external_agent backed by Codex
test_runner       tool_agent or external process
human_approval    approval gate before push or deploy
```

No single CLI agent owns that whole organization.

Nomici should let users compose it.

## Runtime Model

Treat command-driven tools such as Claude Code, Codex, opencode, Aider, and custom commands as:

```text
external_agent
  -> cli_agent runtime
  -> cli_invoke runner
  -> generic CLI Agent adapter
```

Example shape:

```yaml
runtimes:
  implementer_cli:
    kind: cli_agent
    runner: cli_invoke
    workspace: ./workspace
    invoke:
      executable: claude
      args:
        - "-p"
        - "${INPUT}"
    capabilities:
      files_read: true
      files_write: true
      shell: true
      streaming: true
      review: unknown
    trust: untrusted

  reviewer_cli:
    kind: cli_agent
    runner: cli_invoke
    workspace: ./workspace
    invoke:
      executable: codex
      args:
        - "review"
        - "${INPUT}"
    capabilities:
      files_read: true
      files_write: true
      shell: true
      review: true
    trust: untrusted

agents:
  implementer:
    kind: external_agent
    runtime: implementer_cli
    trust: untrusted

  reviewer:
    kind: external_agent
    runtime: reviewer_cli
    trust: untrusted
```

This is illustrative. The runner must validate the installed CLI surface on the user's machine and support static configuration for tools whose capabilities cannot be probed reliably.

## Common Interface

The common interface is intentionally narrow:

```text
task input
  -> command invocation
  -> workspace
  -> stdout/stderr or structured output
  -> exit code
  -> changed files / artifacts
```

Nomici does not need to understand each tool's internal model provider, prompt format, memory, approval system, or tool-call loop.

Those are data-plane internals.

## CLI Runner Contract v0.1

Private bootstrap implementation note:

- The first executable path is AgentSpec-first: define a `cli_agent` runtime in `nomici.yaml`, then run it with `nomici agent run <external_agent>`.
- `nomici runtime inspect <runtime_id>` can inspect configured runtimes.
- YAML mutation commands such as `nomici runtime add ...` are deferred until the Runtime Registry edit path exists, so the bootstrap does not introduce a second source of truth.

The CLI Agent Runner is a process runner contract. It is not the same surface as an HTTP adapter, although Gateway normalizes its result into the same run/trace model.

### Invoke Input

Runner input:

```json
{
  "run_id": "run_01H",
  "task_id": "task_01H",
  "agent_id": "implementer",
  "workspace": "./workspace",
  "prompt": "Implement login middleware.",
  "shared_context": {
    "project": [],
    "run": [],
    "handoff": {
      "summary": "Architect selected stateless JWT auth."
    }
  },
  "artifacts": [],
  "timeout_seconds": 1800,
  "env_refs": ["ANTHROPIC_API_KEY"]
}
```

The runner renders this through the configured command template.

Template variables:

- `${INPUT}`: user/task prompt plus rendered task briefing
- `${PROMPT}`: raw prompt only
- `${BRIEFING}`: rendered Shared Context briefing only
- `${RUN_ID}`
- `${TASK_ID}`
- `${WORKSPACE}`

Rules:

- Prefer argument arrays over shell strings.
- Shell expansion is disabled unless `shell: true` is explicitly configured.
- Environment variables are passed by allowlist or secret reference only.
- Shared Context is injected as a bounded task briefing, not as hidden runtime memory.

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

### Output Collection

Runner output:

```json
{
  "status": "completed",
  "exit_code": 0,
  "stdout_ref": "artifact_stdout_01H",
  "stderr_ref": "artifact_stderr_01H",
  "summary": "Implemented login middleware and added tests.",
  "changed_files": [
    "server/auth/middleware.go",
    "server/auth/middleware_test.go"
  ],
  "diff_ref": "artifact_diff_01H",
  "context_snapshot": {
    "summary": "Implementation is complete; refresh-token rotation remains out of scope.",
    "open_issues": [
      "Refresh-token rotation is not implemented."
    ],
    "artifact_refs": ["artifact_diff_01H"]
  }
}
```

Rules:

- Exit code `0` maps to `completed` unless parsing or policy marks the result unsafe.
- Non-zero exit codes map to `failed` with stdout/stderr artifacts preserved.
- If the CLI emits declared structured JSON, parse it.
- Otherwise, stdout is the agent response and stderr is diagnostic output.
- Capture pre/post workspace diff when the workspace is a Git repo.
- If the workspace is not a Git repo, capture a file manifest diff where feasible.
- Store large stdout/stderr by artifact reference, not inline trace payload.

### Timeout and Cancel

Defaults:

- timeout: 30 minutes unless runtime config overrides it
- graceful cancel: send `SIGTERM` or platform equivalent
- grace period: 10 seconds
- forced cancel: send `SIGKILL` or platform equivalent

Rules:

- Cancellation must emit `adapter.cancelled` or `adapter.failed`.
- Partial stdout/stderr and diffs should be preserved as artifacts when safe.
- The runner should terminate the process group, not only the direct child, when supported by the OS.

### Concurrency and Workspace Locks

Default v0.1 behavior:

- one mutable CLI agent invocation per workspace at a time
- concurrent read-only invocations are allowed only when the runtime declares `files_write: false`
- mutating parallel work should use isolated workspaces, branches, or worktrees

Gateway should maintain a workspace lock for mutable CLI runs.

If a second mutable invocation targets a locked workspace, policy should either queue it or fail clearly with remediation.

### Normalization

After process execution, Gateway normalizes the result into:

- run/task status
- adapter trace events
- artifacts
- context snapshot candidate
- policy/audit records

This keeps CLI agents first-class without pretending they implement the HTTP adapter contract internally.

## Adapter Modes

The CLI Agent Runner should support modes, not one hard-coded command.

Minimum modes:

- `invoke`: send a prompt and collect final output
- `stream`: collect structured or line-oriented streaming output if supported
- `review`: ask the runtime to review a diff, branch, or workspace if supported
- `resume`: resume a prior session if supported
- `capabilities`: probe installed command, version, non-interactive support, declared capabilities, and supported safety flags

Optional modes:

- `apply_patch`: apply an agent-produced patch through a controlled path
- `mcp_server`: expose the agent as an MCP server when the CLI supports that mode
- `sandboxed_exec`: run with a CLI-native sandbox option where available

## Capability Probe

Adapter setup should probe where possible:

- executable exists
- version command works
- non-interactive invocation exists
- working directory can be set
- output format options
- streaming options
- review command support
- approval/sandbox options
- MCP server mode support
- resume/session support

Capability values should be tri-state:

- `true`
- `false`
- `unknown`

Do not assume a capability from the product name.

Some CLI agents will not expose reliable probing. For them, v0.1 should allow conservative static declarations in the runtime config and mark unverified claims as `unknown` or `declared`.

Example profiles:

```text
claude_code
codex
opencode
aider
cline
continue
custom
```

Profiles are presets over `cli_agent`, not separate core adapters.
Profiles for editor-native tools require an actual command-driven surface before they are enabled.

## Safety Model

CLI agents are powerful local runtimes. They may read files, edit files, run shell commands, and call network services.

Default policy:

- treat them as `untrusted` external agents
- run inside an explicit workspace
- do not pass broad secrets by default
- do not use flags that bypass permissions or sandboxing
- require approval before git push, PR creation, deploy, destructive writes, or writes outside allowed scopes
- record prompts, summaries, outputs, changed files, tool requests, and artifacts through Trace where possible

Important limitation:

If a local CLI agent executes tools internally, Nomici may not see every internal tool call. Nomici should be honest about this.

Mitigations:

- workspace isolation
- branch or worktree isolation
- pre/post diff capture
- command transcript capture where available
- policy gates before external side effects
- human approval before publishing changes

## Developer Team Pack

The `developer-team` pack should make CLI agent runtimes first-class.

Recommended default graph:

```text
product_pm
  -> architect
  -> implementer
  -> reviewer
  -> test_runner
  -> human_approval
```

Recommended runtime mapping:

```text
product_pm      gateway_agent
architect       gateway_agent or local model-backed agent
implementer     Claude Code, Codex, opencode, Aider, Hermes, OpenClaw, or another external CLI agent
reviewer        Codex, Claude Code, opencode, Aider, or another review-capable external CLI agent
test_runner     tool_agent
approval        approval gate
```

The pack should not require any specific CLI agent. It should detect installed profiles and offer them as runtime choices.

If no supported CLI agent is installed, the pack can fall back to:

- OpenAI-compatible endpoint
- local Ollama model
- manual task handoff
- disabled implementer/reviewer nodes with clear setup guidance

Community packs should be able to add a new CLI agent by YAML configuration alone:

- declare a `cli_agent` runtime
- provide an `invoke` command template
- declare conservative capabilities
- attach workspace and policy scopes
- map it to an `external_agent`

No Go adapter should be required for ordinary command-driven agents.

## Priority

v0.1 should prioritize the generic CLI Agent Runner because it matches the target user:

- developers already using local AI coding agents
- teams that want governed code changes
- users who want one graph across multiple vendor tools
- users who want trace and approval around agent-produced changes

Recommended priority:

1. OpenAI-compatible model/agent endpoint adapter.
2. Ollama/local model provider.
3. Generic CLI Agent Runner.
4. CLI agent profiles for Codex, Claude Code, opencode, Aider, custom commands, and editor-native tools only where they expose command-driven automation.
5. Hermes/OpenClaw endpoint entries.

Hermes and OpenClaw remain important, but CLI agents should be explicit first-class examples of the data-plane runtime strategy.

This also reduces pressure to implement `gateway_agent` first. A convincing v0.1 demo can use existing CLI agents for implementation and review while Nomici supplies graph coordination, policy gates, trace, and approvals.

## Non-Goals

v0.1 should not:

- reimplement CLI agent behavior
- depend on any one CLI agent being installed
- bypass their safety models
- claim full internal trace visibility into their private tool calls
- normalize every CLI-specific feature into core
- treat vendor-specific auth as Nomici-owned secrets unless configured by reference

## Tests

Adapter contract tests should include:

- fake CLI fixture
- command-not-found error
- unsupported non-interactive mode
- structured output parsing
- streaming output parsing where available
- workspace path enforcement
- redaction of env and auth values
- diff capture before and after invocation
- cancellation behavior
- capability probe snapshots
