# Pack System Design

## Purpose

Packs make Nomici useful without bloating core.

An agent can be installed as an atomic extension. A pack can distribute one agent, a team of agents, tool definitions, provider definitions, runtime adapters, eval rubrics, examples, and tests.

## Extension Granularity

```text
Agent Definition
  A single reusable agent.

Agent Pack
  A coordinated group of agents, edges, tools, policies, examples, and tests.

Pack
  General distribution unit for providers, tools, agents, runtimes, templates, and evals.
```

Pack types:

- `provider_pack`
- `tool_pack`
- `agent_pack`
- `runtime_adapter_pack`
- `template_pack`
- `eval_pack`

## Pack Manifest Schema

```yaml
id: developer-team
name: Developer Team
version: 0.1.0
kind: agent_pack
description: Product, architecture, implementation, review, and test agents.
publisher: NomiciAI
license: Apache-2.0
requires:
  nomici: ">=0.1.0"
providers:
  required_capabilities:
    - streaming
    - tool_calling
permissions:
  filesystem:
    read:
      - ./workspace
    write:
      - ./workspace
  shell:
    mode: approval
agents:
  entrypoints:
    - product_pm
  includes:
    - product_pm
    - planner
    - researcher
    - coder
    - reporter
roles:
  - id: product_pm
    purpose: Coordinate the run, keep the user goal explicit, and decide the ordered handoff path.
    required_skills:
      - planning
    model_preference: default
    handoff_mode: sequential
    output_contract:
      kind: coordination_brief
      required:
        - goal
        - role_sequence
        - risks
graph:
  fragment: graph.yaml
tests:
  - tests/smoke.yaml
trust:
  level: official
```

The `developer-team` pack should treat Claude Code, Codex, opencode, Aider, Cline, Continue, Hermes, OpenClaw, and similar tools as optional external runtimes for implementer/reviewer roles. Command-driven tools use `cli_agent`; editor-native tools need an automation surface or future sidecar. The pack should not require any one tool to be installed.

Adding a command-driven CLI agent should usually require only pack YAML: a `cli_agent` runtime, an `invoke` command template, conservative capabilities, policy scopes, and an `external_agent` mapping.

Required:

- `id`
- `name`
- `version`
- `kind`
- `description`
- `requires`
- `permissions`

Optional:

- `publisher`
- `license`
- `providers`
- `tools`
- `agents`
- `roles`
- `graph`
- `examples`
- `tests`
- `trust`
- `docs`

`roles` is declarative ownership metadata. Gateway may use it to create ordered task ledger records and display role purpose, expected tools, selected skills, handoff mode, model/runtime preference, and output contract. Gateway must not hardcode bundled role names; packs own those details.

## v0.1 Trust Root

Private bootstrap implementation note:

- `developer-team` is the first bundled official pack.
- The default install path writes a model-backed `product_pm` coordinator entrypoint plus planner, researcher, coder, and reporter role agents into `nomici.yaml`.
- The bundled manifest exposes role metadata, and pack install records that role metadata under `extensions.packs`.
- The installer chooses an existing model provider profile, or a user-selected profile via `--model`.
- Optional CLI implementer/reviewer/test-runner roles stay as manifest metadata until pack install can safely prompt for concrete local runtime commands.
- The installed pack is recorded in `extensions.packs`.

`trust.level` in a pack manifest is not authoritative in v0.1.

Until signed distribution exists:

- `official` only applies to packs bundled with the Nomici binary or listed in a compiled official pack index
- local directory packs are treated as `local` or `untrusted`
- a local manifest claiming `trust.level: official` must not grant privileges
- Console may show the claim as informational only
- permission review is still required for official packs

The `trust` field remains useful as reserved metadata, but Gateway must not trust it without a trust root.

## Pack Install State Machine

```text
discovered
  -> inspected
  -> manifest_validated
  -> dependencies_checked
  -> permissions_reviewed
  -> approved
  -> installed
  -> smoke_tested
  -> active
```

Failure states:

- `invalid_manifest`
- `unsupported_version`
- `missing_provider`
- `missing_tool_dependency`
- `permission_denied`
- `install_failed`
- `smoke_test_failed`

## Permission Review

Before install, Nomici shows:

- filesystem scopes
- shell access
- network access
- MCP servers
- local process runners
- secrets requested by reference
- approval rules
- artifacts written

Packs cannot silently:

- run shell scripts
- install system packages
- read secrets
- write outside workspace
- enable remote access
- publish artifacts externally

## Pack Composition

Multiple packs may be installed in one project.

Conflict rules:

- duplicate agent IDs are errors unless namespaced or explicitly overridden
- duplicate tool IDs are errors unless same definition hash
- policy rules merge conservatively
- permissions union must be shown to user
- provider requirements must be satisfiable by current model profiles

Namespacing:

```text
developer-team/product_pm
office-ops/document_analyst
```

v0.1 may avoid automatic namespace rewriting and require unique IDs.

## Pack Sources

v0.1 sources:

- bundled official packs
- local directory packs

Deferred:

- remote registry
- signed remote packs
- community marketplace

## CLI

```bash
nomici pack list
nomici pack inspect developer-team
nomici pack install developer-team
nomici pack install ./packs/my-pack
nomici pack remove developer-team
nomici pack validate ./packs/my-pack
nomici pack test ./packs/my-pack
```

## Gateway API

```text
GET  /api/packs
GET  /api/packs/{id}
POST /api/packs/inspect
POST /api/packs/install
POST /api/packs/remove
POST /api/packs/validate
POST /api/packs/test
```

## Trace Events

- `pack.inspected`
- `pack.validated`
- `pack.permission_requested`
- `pack.permission_granted`
- `pack.permission_denied`
- `pack.installed`
- `pack.removed`
- `pack.test_started`
- `pack.test_completed`
- `pack.test_failed`

## Tests

- manifest schema validation
- permission summary golden tests
- install state machine tests
- conflict detection
- local pack fixture install
- official pack smoke tests
