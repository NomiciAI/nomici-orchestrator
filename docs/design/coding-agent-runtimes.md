# Coding Agent Runtimes

## Purpose

Claude Code, Codex, and similar local coding agents are high-value data-plane runtimes for Nomici.

They are not competitors to replace in v0.1. They are exactly the kind of strong local agent runtime that makes an independent control plane useful.

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

The difference is not that Nomici writes code better than Claude Code or Codex.

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

Strong local coding agents create the need for a control plane.

A realistic user may want:

```text
product_pm        gateway_agent using configured provider
architect         gateway_agent using local Ollama
implementer       external_agent backed by Claude Code
reviewer          external_agent backed by Codex
test_runner       tool_agent or external process
human_approval    approval gate before push or deploy
```

No single vendor coding agent owns that whole organization.

Nomici should let users compose it.

## Runtime Model

Treat Claude Code, Codex, and similar tools as:

```text
external_agent
  -> cli_invoke runner
  -> cli_invoke adapter
```

Example shape:

```yaml
runtimes:
  claude_code_impl:
    kind: coding_agent_cli
    runner: cli_invoke
    workspace: ./workspace
    command:
      executable: claude
    capabilities:
      files_read: true
      files_write: true
      shell: true
      streaming: true
      review: unknown

  codex_review:
    kind: coding_agent_cli
    runner: cli_invoke
    workspace: ./workspace
    command:
      executable: codex
    capabilities:
      files_read: true
      files_write: true
      shell: true
      review: true

agents:
  implementer:
    kind: external_agent
    runtime: claude_code_impl
    trust: untrusted

  reviewer:
    kind: external_agent
    runtime: codex_review
    trust: untrusted
```

This is illustrative. The adapter must discover and validate the installed CLI surface on the user's machine.

## Adapter Modes

The coding-agent CLI adapter should support modes, not one hard-coded command.

Minimum modes:

- `invoke`: send a prompt and collect final output
- `stream`: collect structured or line-oriented streaming output if supported
- `review`: ask the runtime to review a diff, branch, or workspace if supported
- `resume`: resume a prior session if supported
- `capabilities`: probe installed command, version, non-interactive support, and supported safety flags

Optional modes:

- `apply_patch`: apply an agent-produced patch through a controlled path
- `mcp_server`: expose the agent as an MCP server when the CLI supports that mode
- `sandboxed_exec`: run with a CLI-native sandbox option where available

## Capability Probe

Adapter setup should probe:

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

## Safety Model

Coding agents are powerful local runtimes. They may read files, edit files, run shell commands, and call network services.

Default policy:

- treat them as `untrusted` external agents
- run inside an explicit workspace
- do not pass broad secrets by default
- do not use flags that bypass permissions or sandboxing
- require approval before git push, PR creation, deploy, destructive writes, or writes outside allowed scopes
- record prompts, summaries, outputs, changed files, tool requests, and artifacts through Trace where possible

Important limitation:

If a local coding agent executes tools internally, Nomici may not see every internal tool call. Nomici should be honest about this.

Mitigations:

- workspace isolation
- branch or worktree isolation
- pre/post diff capture
- command transcript capture where available
- policy gates before external side effects
- human approval before publishing changes

## Developer Team Pack

The `developer-team` pack should make coding-agent runtimes first-class.

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
implementer     Claude Code, Codex, Hermes, OpenClaw, or another external coding agent
reviewer        Codex, Claude Code, or another review-capable external agent
test_runner     tool_agent
approval        approval gate
```

The pack should not require Claude Code or Codex. It should detect them and offer them as high-quality runtime choices.

## Priority

v0.1 should prioritize coding-agent CLI adapters because they match the target user:

- developers already using local AI coding agents
- teams that want governed code changes
- users who want one graph across multiple vendor tools
- users who want trace and approval around agent-produced changes

Recommended priority:

1. OpenAI-compatible model/agent endpoint adapter.
2. Ollama/local model provider.
3. Generic coding-agent CLI adapter.
4. Codex CLI profile.
5. Claude Code CLI profile.
6. Hermes/OpenClaw endpoint entries.

Hermes and OpenClaw remain important, but Claude Code and Codex should be explicit first-class examples of the data-plane runtime strategy.

## Non-Goals

v0.1 should not:

- reimplement Claude Code or Codex behavior
- depend on either tool being installed
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
