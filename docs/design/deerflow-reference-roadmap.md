# DeerFlow Reference Capability Roadmap

## Purpose

DeerFlow is a useful reference because it treats long tasks as a full workspace rather than a single chat turn. The public DeerFlow 2.0 material highlights the same product shape Nomici needs: long-horizon tasks, subagents, sandboxed filesystem execution, progressive skills and tools, memory, message gateways, search/fetch/knowledge connectors, human-in-the-loop editing, and multimodal outputs.

Nomici should include these capability families, but not by turning Gateway into a hidden monolithic agent framework. Nomici remains the local-first control plane:

- Gateway owns coordination, sessions, policy, approvals, traces, artifacts, task ledgers, and message normalization.
- Runtimes own their native reasoning loops, tools, memories, and execution engines.
- Packs and AgentSpec expose portable composition.
- Adapters connect specialized systems such as Codex, Claude Code, Hermes, OpenClaw, LangGraph, OpenAI Agents SDK, MCP servers, search providers, knowledge stores, and sandbox providers.

## Reference Mapping

| DeerFlow reference capability | Nomici capability family | Nomici ownership boundary |
| --- | --- | --- |
| Long-horizon tasks | Run sessions, task ledger, resumable message stream | Gateway records state; runtimes execute work |
| Coordinator / Planner / Researcher / Coder / Reporter | Graph-native roles and starter packs | Packs define roles; Gateway schedules bounded tasks |
| Docker / AIO sandbox with browser, shell, file, MCP, VSCode server | Sandbox provider interface and workspace records | Sandbox provider allocates execution; policy gates mutation |
| Progressive skills | Skill registry, pack-installed skills, on-demand briefing injection | Gateway indexes skills; adapters load only selected skill context |
| Tool extension | Tool registry, tool broker, MCP/tool packs | Tool calls go through policy, approval, trace, and redaction |
| Long / short-term memory | Shared Context, memory promotion, session summaries | Nomici stores curated context; runtime-native memory stays private |
| Telegram / Slack / Feishu / Lark channels | Message Gateway channels | Channels create the same run/session records as CLI and Console |
| Search/fetch/knowledge connectors | Research tool packs and knowledge adapters | Connectors are explicit tools with risk, auth, and trace contracts |
| Human-in-the-loop planning and report editing | Clarification checkpoints, plan edits, artifact editors | User edits become traceable run messages and artifact revisions |
| Reports, podcasts, TTS, presentations | Artifact-producing packs | Packs own media generation; Gateway tracks artifacts and approvals |

## PR Sequence

The sequencing below keeps each PR reviewable and testable. Later PRs should not bypass earlier data model work.

### PR A: Run Sessions And Task Ledger

Goal: make long tasks durable before adding more execution paths.

Scope:

- Add `run_sessions` storage with title, status, source channel, active run, created/updated timestamps.
- Add `tasks` storage with parent task, assigned agent, status, context snapshot, artifacts, approvals, and timestamps.
- Emit trace events when task state changes.
- Expose read APIs for session and task summaries.
- Update Console to show task ledger from storage instead of inferring stages only from trace.

Acceptance:

- A run can be refreshed and still show session/task state.
- Sequential handoff creates visible task records.
- Failed/blocked/approval-waiting states are durable.

### PR B: Graph-Native Subagent Roles

Goal: turn the DeerFlow role pattern into Nomici packs without hardcoding roles in Gateway.

Scope:

- Extend `developer-team` into role variants: coordinator, planner, researcher, coder, reporter.
- Add pack manifest metadata for role purpose, required tools, and default handoff mode.
- Support sequential planner -> worker -> reporter chains using existing handoff execution.
- Add eligibility checks and clear unsupported messages for parallel/fallback branches.

Acceptance:

- A starter pack exposes a single entrypoint but renders the subagent path.
- Each role receives bounded Shared Context.
- Console and trace show which role owned each task.

### PR C: Sandbox Provider Interface

Goal: make sandbox setup real enough to support local and container-backed execution.

Scope:

- Define sandbox provider types: `local`, `container`, `remote`.
- Persist sandbox/workspace records per run and per agent where needed.
- Add provider detection for Docker, Podman, and Apple Container.
- Add artifact roots, cleanup status, and workspace path mapping.
- Keep local execution as default; make container execution opt-in until adapter support is proven.

Acceptance:

- `nomici setup --sandbox container` creates a sandbox record when a run starts.
- `nomici doctor` distinguishes configured intent from executable provider availability.
- Trace and Console show workspace, artifacts root, and cleanup state.

### PR D: Skills Registry And Progressive Loading

Goal: make skills visible, installable, and loaded only when relevant.

Scope:

- Add AgentSpec fields for `skills` at project, agent, and run levels.
- Add pack manifest skill metadata: name, description, triggers, files, required tools, risk.
- Add `nomici skill list`, `nomici skill inspect`, and setup/install wiring through packs.
- Inject selected skill briefings into model/CLI runtime requests without dumping all skills into context.

Acceptance:

- Installed skills are visible in CLI, Console, and graph snapshots.
- A run can enable a skill explicitly.
- Trace records which skills were selected.

### PR E: Tool Registry, Broker, And Default Research Tools

Goal: expose useful default tools while keeping mutation policy-controlled.

Scope:

- Add tool registry entries for search, fetch, file read/write, bash, MCP, and knowledge adapters.
- Implement read-only search/fetch tool contracts first.
- Add provider catalog entries for DuckDuckGo, Brave, Tavily, Searx/SearxNG, Jina Reader, and InfoQuest-style fetch providers.
- Route file write and bash through policy approval and sandbox constraints.

Acceptance:

- Tools declare auth, network, filesystem, and mutation risk.
- Read-only search/fetch emits redacted trace events.
- File write/bash cannot run without policy approval in untrusted mode.

### PR F: Knowledge And Memory Bridge

Goal: support long-term continuity without importing raw runtime memory.

Scope:

- Add memory scopes: project, user preference, run summary, decision, artifact summary.
- Add explicit promotion from run/session output into Shared Context.
- Add knowledge connector interfaces for Qdrant, Milvus, RAGFlow, Dify, and similar stores as planned adapters.
- Add CLI/Console review flow before durable memory promotion.

Acceptance:

- A completed run can propose memory entries.
- User approval is required before promotion.
- Runtime-native memory is not scraped by default.

### PR G: Message Gateway Channels

Goal: let non-Console channels create the same run sessions.

Scope:

- Add channel config schema for Slack, Telegram, and Feishu/Lark.
- Normalize inbound channel messages into run session creation.
- Route outbound run updates through channel adapters with redaction and rate limits.
- Keep channels disabled by default and local-only unless explicitly configured.

Acceptance:

- CLI, Console, and channel-created tasks share the same session/task store.
- Secrets and oversized artifacts are never sent to channels by default.
- `nomici doctor` reports channel readiness.

### PR H: Human-In-The-Loop Planning And Artifact Editing

Goal: support clarification, plan edits, and report revision before final delivery.

Scope:

- Add checkpoint task state for `needs_clarification` and `plan_review`.
- Let users edit or approve a plan before execution continues.
- Track artifact revisions and user edits.
- Add Console report/editor surface after artifact metadata is durable.

Acceptance:

- A run can pause for clarification or plan approval.
- User edits are stored as run messages and artifact revisions.
- Execution resumes from the approved plan.

### PR I: Multimodal Artifact Packs

Goal: support non-text outputs as packs, not core assumptions.

Scope:

- Add artifact types for report, slide deck, audio, transcript, image set, and dataset.
- Define pack contracts for TTS, podcast, and presentation generation.
- Keep provider-specific media generation behind adapter/tool contracts.

Acceptance:

- Artifact metadata captures type, source task, files, and review state.
- Console can show generated artifact references.
- Media packs can be installed without changing Gateway core.

## Cross-Cutting Requirements

Every PR in this roadmap must include:

- storage migrations or explicit reason none are needed
- trace events for new state transitions
- policy and redaction behavior for side effects
- CLI tests for setup/doctor/introspection paths
- Gateway handler tests for new APIs
- Console behavior that degrades clearly when a capability is configured but unavailable
- documentation updates and security notes

## Guardrails

- Do not make Gateway run unbounded autonomous loops.
- Do not run parallel agents until task ledger, workspace locking, and budget controls exist.
- Do not treat message streams as the source of truth.
- Do not store raw API keys, channel tokens, private runtime memory, or unredacted tool payloads.
- Do not expose public network channels without explicit auth, origin, and deployment guidance.
