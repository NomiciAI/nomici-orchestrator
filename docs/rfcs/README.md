# RFC Index

RFCs are design history. They capture reasoning, alternatives, and draft decisions.

Current authoritative summaries live in:

- `docs/architecture.md`
- `docs/development-plan.md`
- `ROADMAP.md`

## RFCs

| RFC | Title | Status | Purpose |
| --- | --- | --- | --- |
| 0001 | Product Scope | Draft | Defines product positioning, v0.1 scope, users, goals, and non-goals. |
| 0002 | Architecture | Draft | Defines the initial Gateway-centered architecture. |
| 0003 | AgentSpec v0.1 | Draft | Defines the first version of `nomici.yaml`. |
| 0004 | Security Model | Draft | Defines local-first security defaults, trust zones, approvals, and audit. |
| 0005 | Technology Stack Decision | Draft | Selects Go core, TypeScript Console, pnpm workspace, and Makefile commands. |
| 0006 | Control Plane Architecture | Draft | Defines Nomici as a control plane/designer, not a multi-agent framework. |
| 0007 | AgentGraph IR and Adapter Contract | Draft | Defines IR, adapter contract, reconciler state, trace schema, and eval hooks. |
| 0008 | First-Run Experience, Provider Setup, and Packs | Draft | Defines provider setup, packs, first-run flow, and extension model. |
| 0009 | Application and Engineering Architecture | Draft | Consolidates product modules, engineering modules, storage, API, and phases. |

## Current Decisions

The current implementation direction is:

- Nomici is a local-first Agent Control Plane and Designer.
- Core manages Tool Packs instead of bundling heavy Office/Browser runtimes.
- AgentGraph IR is internal in v0.1.
- `gateway_agent` is the minimal Gateway-run coordinator loop; `native_agent` is not a v0.1 public kind.
- Agent is the atomic extension unit.
- Pack is the distribution and composition unit.
- Provider setup is a v0.1 core feature.
- Gateway is the only control plane.
- Claude Code, Codex, opencode, Aider, custom commands, and editor-native agents with automation surfaces are high-priority data-plane runtimes through a generic CLI Agent Runner.
- Run Engine is lightweight; durable execution belongs to data-plane runtimes.
- Agent-native memory belongs to runtimes; Nomici provides Shared Context for cross-agent handoff and long-running task continuity.
- Side-effecting tools go through Policy, Approval, and Trace.
- Official pack trust requires a bundled pack or compiled official index until signatures exist.
- Console setup requires a Gateway process; CLI-first setup is the hard path unless bootstrap Gateway mode lands.
- SQLite is default; Postgres is later.
- v0.1 does not include remote/team/multi-user mode.
- v0.1 targets first-run useful and safe partial autonomy, not unsupervised full autonomy.
