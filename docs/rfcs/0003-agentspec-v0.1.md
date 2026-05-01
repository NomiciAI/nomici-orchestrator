# RFC 0003: AgentSpec v0.1

Status: Draft
Date: 2026-04-30
Target release: Nomici Orchestrator v0.1

## Summary

AgentSpec is the versioned YAML format used by Nomici Orchestrator to define an agent organization.

The project config file is:

```text
nomici.yaml
```

AgentSpec v0.1 should define desired state only. Runtime observations such as PIDs, current health, active run IDs, pending approvals, and logs belong in Gateway state, not in `nomici.yaml`.

## Design Principles

Human-readable:

Users should be able to review and edit the spec in a normal editor.

Strictly validated:

Unknown fields should fail validation unless explicitly allowed in an `extensions` map.

Secret-safe:

Secrets must be referenced, not embedded.

Versioned:

Every spec must include a `version` field.

Portable:

The same file should work across machines when dependencies, env vars, and local paths are available.

Control-plane oriented:

The spec describes the organization, graph, policies, and deployment intent. It should not leak runtime internals.

## Top-Level Shape

Required top-level fields:

- `version`
- `project`

Optional top-level fields:

- `models`
- `runtimes`
- `agents`
- `tools`
- `mcp`
- `edges`
- `policies`
- `budgets`
- `deployment`
- `profiles`
- `extensions`

Example:

```yaml
version: "0.1"

project:
  name: ai-application-pm
  description: AI application product manager with architecture and UX subagents.

models:
  gpt:
    kind: openai_compatible
    base_url: ${OPENAI_BASE_URL}
    api_key_env: OPENAI_API_KEY
    model: gpt-5.5
    capabilities:
      - reasoning
      - tool_calling
      - structured_output
    context_window: 400000

  local_qwen:
    kind: ollama
    base_url: http://127.0.0.1:11434
    model: qwen3:32b
    capabilities:
      - tool_calling
    context_window: 32768

runtimes:
  hermes_coder:
    kind: hermes
    runner: local_process
    profile: coder
    start:
      command: hermes -p coder gateway run
    api:
      kind: openai_compatible
      base_url: http://127.0.0.1:8642/v1
      api_key_env: HERMES_API_KEY
    health:
      url: http://127.0.0.1:8642/health

  openclaw_ops:
    kind: openclaw
    runner: local_process
    workspace: ./workspaces/openclaw-ops
    start:
      command: openclaw gateway run
    api:
      kind: openai_compatible
      base_url: http://127.0.0.1:18789/v1
      api_key_env: OPENCLAW_GATEWAY_TOKEN
    health:
      url: http://127.0.0.1:18789/api/health

agents:
  product_pm:
    kind: gateway_agent
    model: gpt
    role: >
      Brainstorm AI application ideas, define product vision,
      and coordinate architecture, UX, and implementation work.
    subagents:
      - senior_architect
      - uiux_designer
      - hermes_coder
      - openclaw_ops

  senior_architect:
    kind: gateway_agent
    model: local_qwen
    role: Design software architecture.

  uiux_designer:
    kind: gateway_agent
    model: gpt
    role: Design software UX and product flows.

  hermes_coder:
    kind: external_agent
    runtime: hermes_coder
    endpoint: hermes_coder

  openclaw_ops:
    kind: external_agent
    runtime: openclaw_ops
    endpoint: openclaw_ops

edges:
  - from: product_pm
    to: senior_architect
    mode: handoff

  - from: product_pm
    to: uiux_designer
    mode: handoff

  - from: product_pm
    to: hermes_coder
    mode: a2a

  - from: product_pm
    to: openclaw_ops
    mode: a2a

policies:
  approvals:
    require:
      - tool.shell.exec
      - tool.filesystem.write
      - tool.email.send
      - runtime.openclaw_ops.tools.invoke
```

## Project

```yaml
project:
  name: ai-application-pm
  description: AI application product manager with architecture and UX subagents.
```

Fields:

- `name`: required, stable project identifier.
- `description`: optional human-readable project description.

Future fields:

- `tags`
- `owners`
- `homepage`
- `repository`

## Models

Models describe LLM providers and local model servers.

Kinds for v0.1:

- `openai_compatible`
- `openai`
- `anthropic`
- `gemini`
- `ollama`
- `vllm`
- `local_openai_compatible`

Minimum fields:

- `kind`
- `model`

Common fields:

- `base_url`
- `api_key_env`
- `capabilities`
- `context_window`
- `input_cost_per_1m`
- `output_cost_per_1m`
- `fallback`
- `timeout`

Capabilities:

- `vision`
- `tool_calling`
- `structured_output`
- `reasoning`
- `embeddings`
- `streaming`

Rules:

- `api_key` is not allowed.
- `api_key_env` is allowed.
- `${ENV_VAR}` substitution is allowed only for configured fields such as `base_url`.
- Cost metadata is advisory in v0.1.

## Runtimes

Runtimes describe managed or external execution environments.

Kinds for v0.1:

- `hermes`
- `openclaw`
- `openai_compatible`
- `ollama`
- `vllm`
- `process`
- `cli_agent`

Runner kinds for v0.1:

- `local_process`
- `cli_invoke`
- `external`

Future runner kinds:

- `docker`
- `kubernetes`
- `remote_gateway`

Example:

```yaml
runtimes:
  custom_worker:
    kind: process
    runner: local_process
    workspace: ./workers/custom
    start:
      command: python worker.py
    env:
      WORKER_MODE: local
    health:
      command: python healthcheck.py

  implementer_cli:
    kind: cli_agent
    runner: cli_invoke
    workspace: ./workspace
    invoke:
      executable: agent-cli
      args:
        - "${INPUT}"
    capabilities:
      files_read: true
      files_write: true
      shell: true
```

Runtime fields:

- `kind`: required.
- `runner`: required for managed runtimes.
- `workspace`: optional working directory.
- `start.command`: command used by local process runner.
- `invoke.executable`: executable used by `cli_invoke`.
- `invoke.args`: argument template used by `cli_invoke`.
- `stop.command`: optional custom stop command.
- `env`: non-secret environment values.
- `env_from`: references to env files or secret stores.
- `api`: endpoint metadata.
- `health`: HTTP or command health check.
- `restart`: restart policy metadata.
- `permissions`: runtime-specific permission scope.

Rules:

- Runtime names are stable IDs.
- Runtime names must be unique.
- Local process commands should not be shell-expanded by default unless explicitly configured.
- Secrets must be injected by reference.

## Agents

Agents describe canvas nodes that can receive work.

Kinds for v0.1:

- `gateway_agent`
- `external_agent`
- `model_agent`
- `tool_agent`

Future kinds:

- `native_agent`
- `remote_a2a_agent`
- `router_agent`
- `approval_agent`
- `memory_agent`

Gateway agent fields:

- `kind`
- `model`
- `role`
- `instructions`
- `tools`
- `subagents`
- `permissions`
- `budget`

External agent fields:

- `kind`
- `runtime`
- `endpoint`
- `capabilities`
- `trust`

Model agent fields:

- `kind`
- `model`
- `role`

Rules:

- `model` must reference an entry in `models`.
- `runtime` must reference an entry in `runtimes`.
- `subagents` must reference entries in `agents` or compatible runtime-backed agent IDs.
- External agents are untrusted by default unless policy says otherwise.
- `gateway_agent` is a minimal Gateway-run coordinator loop, not durable execution or a full framework runtime.

## Tools

Tools describe direct tools controlled by Nomici or tool-like wrappers.

Kinds:

- `shell`
- `filesystem`
- `http`
- `webhook`
- `mcp_tool`

Example:

```yaml
tools:
  workspace_files:
    kind: filesystem
    roots:
      read:
        - ./workspace
      write:
        - ./workspace/output
    approval:
      write: true
```

v0.1 may keep this section minimal and rely primarily on MCP registry entries plus policy rules.

## MCP

MCP servers describe tool and data providers.

Example:

```yaml
mcp:
  servers:
    filesystem:
      transport: stdio
      command: npx
      args:
        - "@modelcontextprotocol/server-filesystem"
        - ./workspace
      trust: untrusted
      permissions:
        filesystem:
          read:
            - ./workspace
          write:
            - ./workspace/output
```

Fields:

- `transport`: `stdio`, `http`, or `sse`.
- `command`: command for stdio servers.
- `args`: command arguments.
- `url`: URL for remote servers.
- `env`: non-secret environment values.
- `env_from`: secret references.
- `trust`: `trusted`, `untrusted`, or `sandboxed`.
- `permissions`: scope granted to this server.

Rules:

- MCP servers are untrusted by default.
- Tools from untrusted MCP servers require policy evaluation before use.

## Edges

Edges describe graph relationships.

Modes:

- `handoff`
- `a2a`
- `tool_call`
- `mcp`
- `parallel`
- `fallback`
- `approval_required`
- `memory_read`
- `memory_write`

Memory edge rules:

- `memory_read` and `memory_write` may refer to Nomici Shared Context, external memory systems, or runtime-native memory declarations.
- They do not imply Nomici owns agent-native memory.
- v0.1 should execute only Shared Context handoff/read/write operations that Gateway explicitly supports.

Example:

```yaml
edges:
  - from: product_pm
    to: senior_architect
    mode: handoff
```

Rules:

- `from` and `to` must reference valid graph nodes.
- v0.1 can render all edge modes but does not need to execute all modes.
- Unsupported execution modes must fail clearly at run time.

## Policies

Policies define approval, deny, and allow rules.

Example:

```yaml
policies:
  approvals:
    require:
      - tool.shell.exec
      - tool.filesystem.write
      - tool.email.send
      - runtime.openclaw_ops.tools.invoke

  network:
    allow:
      - localhost
      - 127.0.0.1
      - api.github.com
    unknown_host: approval

  filesystem:
    read:
      - ./workspace
    write:
      - ./workspace/output

  shell:
    mode: approval
```

Policy decisions:

- `allow`
- `deny`
- `approval`

Defaults:

- Shell execution: approval.
- Filesystem write: approval.
- Email send: approval.
- Calendar mutation: approval.
- Deployment: approval.
- Git push: approval.
- Unknown network host: approval or deny, depending on workspace policy.
- Remote A2A agent: untrusted.
- MCP server: untrusted.

## Budgets

Budgets are advisory in v0.1.

Example:

```yaml
budgets:
  default:
    max_usd_per_day: 10
    max_tokens_per_run: 200000
```

v0.1 should record budget metadata and emit `budget.exceeded` events where possible, but should not promise precise cost accounting for every external runtime.

## Deployment

Deployment controls Gateway and local runtime settings.

Example:

```yaml
deployment:
  gateway:
    bind: 127.0.0.1
    port: 8787
    auth:
      mode: token
    openai_compatible:
      enabled: false
```

Defaults:

- Gateway bind: `127.0.0.1`
- Gateway port: `8787`
- Auth mode: token
- Remote access: disabled
- OpenAI-compatible API: explicit opt-in unless v0.1 decides otherwise

## Validation

Required validation:

- `version` is supported.
- Required top-level fields exist.
- Unknown fields fail unless under `extensions`.
- References resolve.
- Names are unique.
- Secret values are not embedded in forbidden fields.
- Local paths are normalized but not required to exist during pure schema validation.
- Runtime start commands are present for managed local processes.
- Policy action identifiers are valid.

Commands:

```bash
nomici spec validate nomici.yaml
nomici spec schema > nomici.schema.json
nomici spec migrate --from 0.1 --to 0.2
```

## Open Questions

- Should `version` be `"0.1"` or `0.1`? Recommendation: string.
- Should unknown fields fail immediately in v0.1? Recommendation: yes.
- Should `tools` and `mcp.servers` be separate? Recommendation: yes; MCP servers are providers, tools are capabilities.
- Should model provider-specific fields be inlined or nested under `provider`? Recommendation: inline for v0.1, revisit after real examples.
- Should Gateway OpenAI-compatible API be represented under `deployment.gateway.openai_compatible` or `gateway` top-level? Recommendation: keep under `deployment`.
