# Development Plan

This is the current authoritative development plan for the private bootstrap phase.

The plan is intentionally staged. Nomici should prove one useful vertical slice before expanding protocol depth, framework adapters, team mode, or marketplace features.

Implementation-oriented design notes live in `docs/design/`.

## Current Phase

Status:

```text
Private bootstrap
Design-first
Minimal scaffold exists
Not public
Not ready for user installation
```

Repository policy:

- Direct maintainer pushes to `main` are allowed during private bootstrap.
- Force-push and branch deletion remain disabled.
- Before quiet public, restore strict branch protection with PR review and CODEOWNERS review.

## v0.1 Goal

Nomici v0.1 should prove:

```text
provider setup -> pack install -> Gateway -> runtime/agent registry
  -> first run -> trace -> approval/artifact review
```

v0.1 is not a full autonomous agent platform. It is a local-first control plane that can run useful starter packs.

## Proof Slice

Before the full v0.1 product slice, the project should prove a smaller hard slice:

```text
provider setup
  -> OpenAI-compatible or Ollama model profile
  -> Gateway-mediated invocation
  -> trace event storage
  -> secret redaction
  -> health/status visibility
```

This hard slice validates the Gateway, provider setup, adapter invocation, storage, trace, and security path without requiring packs, graph canvas editing, runtime reconciliation, approval queue, A2A sidecars, or MCP proxying.

## Delivery Tiers

Hard delivery:

- provider setup for OpenAI-compatible and Ollama
- model profile storage without raw secrets
- one Gateway-mediated invocation path
- trace event store
- basic Gateway health/status
- documented security defaults

Product delivery:

- pack manifest validation
- pack permission review
- AgentGraph compile/validate
- basic runtime observed state
- generic coding-agent CLI adapter
- run records and trace timeline
- approval record model
- first useful official bundled pack

Degradable delivery:

- complete Console setup wizard
- rich visual canvas editing
- runtime auto-restart
- non-trivial approval scopes
- full MCP proxy
- A2A sidecars
- Office/browser packs
- signed remote pack distribution

## Phase 0: Foundation

Status: in progress

Goals:

- repository governance
- RFCs
- open-source guardrails
- Go CLI/Gateway scaffold
- Web Console scaffold
- basic health endpoint
- contributor commands

Exit criteria:

```bash
make build
./bin/nomici --version
./bin/nomici gateway run
curl http://127.0.0.1:8787/api/health
```

Also required:

- README accurately marks implemented vs intended commands.
- `make test` passes.
- `make lint` or equivalent baseline passes.

## Phase 1: Provider Setup

Goals:

- provider catalog scaffold
- generic OpenAI-compatible provider setup
- Ollama setup
- model test prompt
- capability profile
- secret reference handling
- provider doctor checks

Commands:

```bash
nomici model setup
nomici model test
nomici model list
nomici model doctor
```

Exit criteria:

- User can configure at least one OpenAI-compatible endpoint.
- User can configure Ollama.
- Raw secrets are not written to `nomici.yaml`.
- Capability profile supports `true`, `false`, and `unknown`.
- Provider test emits trace events.
- A single Gateway-mediated model invocation can be traced without requiring packs or full AgentGraph execution.

## Phase 2: Packs and AgentGraph

Goals:

- pack manifest schema draft
- AgentSpec to AgentGraph compile
- graph validation
- pack permission review
- `ai-application-pm` pack
- `developer-team` pack scaffold

Commands:

```bash
nomici pack list
nomici pack inspect developer-team
nomici pack install developer-team
nomici graph validate
```

Exit criteria:

- User can inspect pack permissions before install.
- User can install an official local pack.
- Pack can install one single-agent definition.
- Pack can install one multi-agent team graph.
- AgentGraph IR remains internal.
- AgentGraph starts from the minimal fields needed by implemented adapters and grows from implementation evidence.
- Unsupported graph edges fail clearly if executed.

## Phase 3: Runtime and Adapters

Goals:

- local process runtime manager
- runtime desired/observed state
- health checks
- logs
- OpenAI-compatible adapter
- generic coding-agent CLI adapter
- Codex CLI profile
- Claude Code CLI profile
- Hermes endpoint entry
- OpenClaw endpoint entry

Commands:

```bash
nomici up
nomici down
nomici ps
nomici runtime logs <runtime>
nomici runtime inspect <runtime>
```

Exit criteria:

- `nomici up` can start configured local process runtimes.
- Runtime state records PID, endpoint, health, and logs.
- OpenAI-compatible endpoint can be invoked through adapter contract.
- At least one installed local coding agent can be represented as an external runtime when available.
- Codex and Claude Code profiles degrade cleanly when their commands are not installed.
- Hermes/OpenClaw can be represented as external runtimes.
- Drift is visible through CLI and Gateway API.

## Phase 4: Runs, Trace, Approval, Artifacts

Goals:

- lightweight run engine
- task ledger
- append-only trace events
- approval queue
- artifact metadata
- JSONL trace export

Commands:

```bash
nomici run <agent> "..."
nomici trace list
nomici trace show <run_id>
nomici trace export <run_id> --format jsonl
nomici approvals list
nomici approvals grant <approval_id>
nomici approvals deny <approval_id>
```

Exit criteria:

- Run creates structured trace events.
- Side-effecting tool requests can require approval.
- Approval decisions are audited.
- Artifacts are stored under workspace `.nomici/`.
- Timeline replay is possible.
- Deterministic re-execution is not promised.

## Phase 5: Console MVP

Goals:

- setup checklist
- provider catalog UI
- model test UI
- pack gallery
- permission review
- canvas view
- runtime status
- runs/traces
- approvals
- artifacts

Exit criteria:

- User can inspect the first-run flow from Console after Gateway starts.
- Console does not receive raw secrets.
- Console renders graph from Gateway API.
- Console can inspect traces and artifacts.
- Full provider setup from Console requires bootstrap Gateway mode; if that mode is not implemented, CLI-first setup remains the hard path.

## Phase 6: Pre-Public Hardening

Goals:

- CI baseline
- branch protection strict mode
- release artifact plan
- install script review
- secret scan
- security docs review
- README status update

Exit criteria:

- Strict branch protection restored.
- `main` requires PR review and CODEOWNERS review.
- GitHub Actions permissions default to read-only.
- Dependabot alerts and security fixes enabled.
- Open-source publication checklist complete.
- README quickstart reflects actual behavior.

## Explicitly Deferred From v0.1

- remote/team/multi-user mode
- Postgres
- OIDC/RBAC
- Kubernetes/Helm
- remote pack registry
- signed pack distribution
- full Office pack
- browser automation pack
- PR creation and CI repair loops
- A2A sidecars
- full MCP proxy
- automated eval scoring
- Docker runner
- marketplace

## Do Not Cut From v0.1

- provider setup
- trace event store
- security defaults
- model profile storage without raw secrets
- one Gateway-mediated invocation path

Do not cut from the v0.1 product slice:

- pack manifest
- AgentGraph compile/validate
- runtime observed state
- approval model
- first-run useful official pack

## Quality Bar

Each phase should include:

- focused tests
- updated docs
- redaction review where secrets may appear
- clear CLI errors with remediation
- no raw secrets in configs, logs, or debug output
- no hidden network calls

## Next Concrete Work

Recommended next implementation task:

```text
Finish and verify Phase 0 scaffold.
```

Scope:

- make current Go/Web scaffold build
- verify `nomici --version`
- verify `nomici gateway run`
- verify `/api/health`
- update README status for implemented scaffold only

Do not start Provider Setup until Phase 0 is passing.
