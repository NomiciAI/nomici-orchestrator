# RFC 0008: First-Run Experience, Provider Setup, and Packs

Status: Draft
Date: 2026-05-01
Target release: Nomici Orchestrator v0.1 and beyond

## Summary

Nomici must balance two goals:

```text
Users should be able to install Nomici and run useful agents quickly.
Developers should see a serious, extensible control-plane architecture.
```

The project should not ship as an empty agent framework that requires users to design everything from scratch. It should ship with:

- guided LLM provider setup
- provider capability probes
- safe secret references
- built-in agent packs
- tool packs for local work
- runtime adapter extension points
- clear pack manifests and permission models

The architecture must stay clean: core control-plane logic should remain separate from heavy provider, office, browser, developer, and personal-ops capabilities.

## Product Goal

Nomici's first-run experience should be:

```bash
nomici init
nomici model setup
nomici pack install developer-team
nomici up
nomici run product_pm "Plan and implement a small web app"
```

The user should not start from an empty canvas. The user should start from a useful, inspectable agent organization.

The developer should see:

- stable pack manifests
- provider setup contracts
- adapter contracts
- toolpack contracts
- trace schemas
- policy and approval hooks
- test fixtures

This is the difference between a demo app and an open-source platform.

## Non-Goals

v0.1 should not:

- support every provider natively
- bundle heavy office runtimes into the core binary
- install untrusted scripts without review
- make telemetry required
- make cloud accounts required
- promise fully autonomous long-running execution without checkpoints and approvals
- make AgentGraph IR a public standard

AgentGraph IR remains an internal contract in v0.1. External users should extend Nomici through packs, adapters, Gateway APIs, and AgentSpec.

## First-Run Experience

The target flow:

```text
Install
  -> Doctor
  -> Provider setup
  -> Choose pack
  -> Review permissions
  -> Start Gateway
  -> Run task
  -> Inspect trace and artifacts
```

CLI shape:

```bash
nomici doctor
nomici model setup
nomici model test
nomici pack list
nomici pack install developer-team
nomici pack inspect developer-team
nomici up
nomici run product_pm "..."
```

Console shape:

- Setup checklist
- Provider catalog
- Model test prompt
- Pack gallery
- Permission review
- First run launcher
- Trace and artifact viewer

Console setup boundary:

- Console is served by Gateway, so Console setup requires a Gateway process.
- The hard v0.1 path is CLI-first setup.
- A bootstrap `nomici gateway run --setup` mode can provide Console setup before runtimes are started.

## Provider Setup

Provider setup should be a first-class feature, not only YAML editing.

Supported setup dimensions:

- provider kind
- base URL
- model name
- API key source
- capabilities
- context window
- cost metadata
- fallback model
- local health check
- test prompt

Provider classes:

- native cloud provider
- OpenAI-compatible endpoint
- local model provider
- provider gateway

## Provider Catalog

Nomici should include a provider catalog.

Catalog entry:

```yaml
id: openai
name: OpenAI
kind: native
auth:
  type: api_key_env
  env: OPENAI_API_KEY
setup:
  required:
    - model
  optional:
    - base_url
models:
  discovery: static_or_api
capabilities:
  probe: true
docs_url: https://platform.openai.com/docs
```

Catalog categories:

- `cloud`
- `local`
- `openai_compatible`
- `gateway`

Initial first-class providers:

- OpenAI
- Anthropic
- Gemini
- OpenRouter
- Ollama
- Codex CLI local auth
- vLLM
- LM Studio
- SGLang
- llama.cpp server
- generic OpenAI-compatible endpoint

Long-tail providers should be supported through gateway adapters such as LiteLLM or OpenRouter where appropriate.

## Provider Setup Wizard

The provider wizard should:

1. Ask which provider to use.
2. Explain what credential is needed.
3. Store only secret references.
4. Discover or validate model names where possible.
5. Probe capabilities.
6. Run a test prompt.
7. Save a model profile.
8. Suggest compatible packs.

Example:

```bash
nomici model setup
? Provider: OpenRouter
? API key env var: OPENROUTER_API_KEY
? Model: anthropic/claude-sonnet-4.5
Testing model...
Capability probe:
  streaming: yes
  tool calling: yes
  structured output: unknown
Saved model profile: openrouter_sonnet
```

Rules:

- Raw API keys must not be written to `nomici.yaml`.
- Console must not display full secret values.
- CLI output must redact secret-looking values.
- Provider test runs must be traceable.
- CLI provider setup can run before Gateway. Console provider setup requires bootstrap Gateway mode.

## Capability Probe

Provider setup should produce a capability profile.

Capability profile:

```yaml
model: openrouter_sonnet
provider: openrouter
capabilities:
  streaming: true
  tool_calling: true
  structured_output: unknown
  vision: false
  reasoning: true
  embeddings: false
limits:
  context_window: 200000
cost:
  input_per_1m: null
  output_per_1m: null
source:
  kind: probed
  checked_at: "2026-05-01T00:00:00Z"
```

Capability values:

- `true`
- `false`
- `unknown`

Unknown should be acceptable. Nomici should not pretend to know capabilities it has not checked.

## Packs

Packs are the user-facing distribution and composition system.

Packs are not the only extension granularity. Nomici should support both atomic extensions and composition extensions.

Core distinction:

```text
Agent Definition
  A single agent as an atomic extension.

Agent Pack
  A coordinated group of agents, edges, tools, policies, examples, and tests.

Pack
  A general distribution unit for providers, tools, agents, runtimes, templates, and evals.
```

Decision:

> Agent is the atomic extension unit. Pack is the distribution and composition unit. Agent Pack is a pack that installs a coordinated group of agents, edges, tools, and policies.

Users and developers should be able to:

- create one new agent
- import one new agent
- publish one reusable agent definition
- install a multi-agent team pack
- compose multiple packs into one project

Single-agent example:

```yaml
agents:
  legal_reviewer:
    kind: gateway_agent
    model: gpt
    role: Review contracts and identify risks.
```

Multi-agent pack example:

```text
contract-review-team
  root: legal_pm
  agents:
    - legal_reviewer
    - risk_analyst
    - redline_writer
    - human_approval
  edges:
    - legal_pm -> legal_reviewer
    - legal_pm -> risk_analyst
    - redline_writer -> human_approval
  tools:
    - office.docx
    - filesystem
  policies:
    - docx write requires approval
```

Agent-to-agent interaction inside an Agent Pack does not always mean A2A.

Use edge kinds precisely:

- `delegates_to` for internal delegation inside a Nomici graph
- `handoff` when control moves from one agent to another
- `agent_as_tool` when an agent is callable as a tool by another agent
- `a2a` when crossing runtime, server, vendor, or remote-agent boundaries through the A2A protocol
- `uses_tool` or `uses_mcp` for tools and MCP servers
- `requires_approval` for human or policy gates

Nomici should not label every agent-to-agent edge as A2A. A2A is the interoperability protocol for heterogeneous or remote agents, not the only internal coordination mode.

Pack types:

- Provider Pack
- Tool Pack
- Agent Pack
- Runtime Adapter Pack
- Template Pack
- Eval Pack

Core rule:

> Nomici core provides the control plane. Packs provide useful domain capabilities.

Packs should be inspectable, versioned, permission-aware, and testable.

## Pack Manifest

Every pack should include a manifest.

Example:

```yaml
id: developer-team
name: Developer Team
version: 0.1.0
kind: agent_pack
description: Product, architecture, implementation, review, and test agents.
requires:
  nomici: ">=0.1.0"
providers:
  required_capabilities:
    - tool_calling
    - streaming
permissions:
  filesystem:
    read:
      - ./workspace
    write:
      - ./workspace
  shell:
    mode: approval
  network:
    unknown_host: approval
agents:
  entrypoints:
    - product_pm
graph:
  fragment: graph.yaml
examples:
  - examples/build-small-web-app.yaml
tests:
  - tests/smoke.yaml
trust:
  level: official
```

Required manifest fields:

- `id`
- `name`
- `version`
- `kind`
- `description`
- `requires`
- `permissions`

Recommended fields:

- `providers`
- `tools`
- `agents`
- `graph`
- `examples`
- `tests`
- `trust`
- `docs`

## Pack Trust Levels

Pack trust levels:

- `official`
- `community`
- `local`
- `untrusted`

Default behavior:

- Official packs may be shown prominently.
- Community packs require warning and review.

v0.1 trust root:

- `official` is authoritative only for packs bundled with the Nomici binary or listed in a compiled official pack index.
- Local directory packs that claim `trust.level: official` are still treated as `local` or `untrusted`.
- Signed remote pack distribution is deferred, so local manifest trust claims do not grant privileges.
- Local packs are user-controlled but still permission-reviewed.
- Untrusted packs cannot auto-run commands.

Packs that include executable scripts, MCP servers, local processes, or network access should require explicit review.

## Tool Packs

Tool Packs provide local capabilities through MCP servers, helper processes, or controlled adapters.

Initial official Tool Packs:

- `office`
- `browser`
- `developer`
- `filesystem`
- `personal-ops`

Tool Pack rule:

> Heavy or risky tools live outside the core binary and are mediated by Gateway policy.

### Office Tool Pack

The `office` pack should support:

- DOCX read, write, comment, redline
- XLSX/CSV read, write, formulas, charts
- PPTX create, edit, render, export
- PDF read, extract, render
- artifact generation
- visual verification where possible

Implementation strategy:

- Use local helper runtimes.
- Prefer MCP tool exposure.
- Detect dependencies with `nomici doctor`.
- Install optional dependencies only after user approval.

Possible dependencies:

- LibreOffice
- Pandoc
- Python document libraries
- Node document libraries
- Poppler for PDF rendering

These must not be bundled blindly into the core binary.

### Developer Tool Pack

The `developer` pack should support:

- filesystem read/write with approval
- shell with approval
- git operations
- test runner
- patch generation
- PR creation later
- CI log ingestion later

This pack should absorb useful patterns from coding-agent orchestrators:

- worktree isolation
- branch-per-agent
- PR review loop
- CI failure repair
- human approval before push/deploy

The pack should treat Claude Code, Codex, opencode, Aider, Cline, Continue, Hermes, OpenClaw, and similar local agents as optional external runtimes. Command-driven tools use `cli_agent`; editor-native tools need an automation surface or future sidecar. They are not dependencies, but they are high-value runtime choices when installed.

## Agent Packs

Agent Packs provide ready-to-run agent organizations.

Initial official Agent Packs:

### Developer Team

Agents:

- Product PM
- Architect
- Implementer
- Reviewer
- Test Runner
- Release Manager
- Human Approval Gate

Use cases:

- implement small feature
- review code
- fix failing tests
- prepare release notes

Suggested runtime mapping:

- Product PM: `gateway_agent`
- Architect: `gateway_agent` or local model-backed agent
- Implementer: Claude Code, Codex, opencode, Aider, Hermes, OpenClaw, or another external CLI agent
- Reviewer: Codex, Claude Code, opencode, Aider, or another review-capable external CLI agent
- Test Runner: tool agent or local process
- Human Approval Gate: approval node before publish actions

### Office Ops

Agents:

- Document Analyst
- Spreadsheet Analyst
- Deck Builder
- PDF Extractor
- File Organizer

Use cases:

- summarize documents
- produce spreadsheet analysis
- create slide draft
- extract structured data from PDF

### Research Team

Agents:

- Researcher
- Browser Agent
- Summarizer
- Citation Checker
- Memory Writer

`Memory Writer` means writing to Nomici Shared Context or to a configured external memory system through an adapter. It does not imply Nomici owns every runtime's internal memory.

Use cases:

- research a topic
- produce brief
- compare vendors
- create cited summary

### Personal Ops

Agents:

- Task Planner
- Email Assistant
- Calendar Assistant
- File Manager
- Approval Gate

Use cases:

- plan week
- draft email
- organize files
- schedule follow-up with approval

### AI Application PM

Agents:

- Product PM
- Architect
- UI/UX Designer
- Coding Agent
- Operator Agent

Use cases:

- design an AI app
- turn idea into architecture
- produce implementation plan

## Long-Running Task Model

Nomici should support long-running tasks as a control-plane concern first.

Core concepts:

- run
- task
- subtask
- assignment
- checkpoint
- artifact
- shared context
- handoff briefing
- approval
- trace
- resume
- cancel

v0.1 model:

```yaml
task:
  id: task_01H
  run_id: run_01H
  title: Implement login flow
  status: in_progress
  assigned_agent: implementer
  checkpoint_ref: chk_01H
  artifacts:
    - patch.diff
  approvals:
    - approval_01H
```

Nomici should not implement a full durable workflow engine immediately. For durable runtime behavior, Nomici can integrate:

- LangGraph
- Google ADK
- CrewAI Flows
- OpenAI Agents SDK sessions
- specialized coding-agent orchestrators

Nomici's value is to make these visible, governable, resumable, and auditable.

Shared context for long-running tasks:

- upstream agents emit context snapshots at handoff boundaries
- downstream agents receive task briefings, artifact summaries, and open issues
- project decisions and user feedback can be promoted into project context
- runtime-native memory stays inside each runtime

## Pack Install Flow

Pack install should be explicit:

```bash
nomici pack inspect developer-team
nomici pack install developer-team
```

Install flow:

1. Fetch or load manifest.
2. Validate manifest.
3. Show permissions.
4. Check provider compatibility.
5. Check local dependencies.
6. Ask for approval for risky setup.
7. Install graph fragments and examples.
8. Register tools and runtimes.
9. Run smoke test if requested.

Packs should never silently:

- install system packages
- run shell scripts
- write outside workspace
- upload local files
- read secrets
- enable remote access

## Pack Development Experience

Developers should be able to create a pack without modifying Nomici core.

Planned commands:

```bash
nomici pack init my-pack
nomici pack validate ./my-pack
nomici pack test ./my-pack
nomici pack publish ./my-pack
```

Pack repository shape:

```text
my-pack/
  nomici-pack.yaml
  graph.yaml
  tools/
  examples/
  tests/
  README.md
```

Developer extension points:

- provider catalog entries
- adapter implementations
- MCP tool definitions
- agent graph fragments
- eval rubrics
- templates

## Security Model

Packs are supply-chain objects.

Security requirements:

- Manifest validation.
- Permission review.
- Secret redaction.
- Trust labels.
- Checksums or signatures for remote packs later.
- Audit events for install and activation.
- No automatic execution from untrusted packs.

Pack events:

- `pack.inspected`
- `pack.installed`
- `pack.updated`
- `pack.removed`
- `pack.permission_requested`
- `pack.permission_granted`
- `pack.permission_denied`

## v0.1 Commitments

v0.1 should implement:

- Provider catalog scaffold.
- `nomici model setup` design and at least one provider path.
- Generic OpenAI-compatible provider config.
- Ollama provider config.
- Provider test prompt.
- Pack manifest schema draft.
- Built-in `ai-application-pm` pack.
- Built-in `developer-team` pack scaffold.
- Generic CLI Agent Runner profile for developer-team runtimes.
- Handoff context snapshots for developer-team runs.
- Pack permission review in CLI, even if simple.

v0.1 may defer:

- remote pack registry
- signed pack distribution
- full office pack implementation
- full browser automation
- PR creation
- durable task engine
- automated eval scoring

## Documentation Requirements

Docs must make clear:

- Nomici core is not a bag of scripts.
- Packs are inspectable and permission-aware.
- Provider setup stores references, not raw secrets.
- Office and developer packs can touch local files and require approval.
- Community packs are not trusted by default.
- Long-running autonomy requires checkpoints and approvals.

## Open Questions

- Should official packs live in the main repository or a separate `nomici-packs` repository?
- Should pack manifests be embedded in AgentSpec or separate files?
- Should provider catalog updates ship with Nomici releases or update independently?
- Should `office` pack be official in v0.1 or experimental?
- Should pack install support offline/local-only mode from the first release?
- Should `nomici model setup` write to global profile config or project `nomici.yaml` by default?
