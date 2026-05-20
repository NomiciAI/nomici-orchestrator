# DeerFlow Reference Capability Roadmap

## Purpose

This document translates DeerFlow 2.0's product shape into Nomici's control-plane roadmap.

Sources reviewed on 2026-05-20:

- DeerFlow intro page: `https://deerflow.one/intro`
- Local DeerFlow checkout: `/Users/stephen/Documents/my-workspace/deer-flow`
- DeerFlow local docs and implementation references:
  - `README_zh.md`
  - `backend/docs/ARCHITECTURE.md`
  - `backend/docs/API.md`
  - `backend/packages/harness/deerflow`
  - `backend/app/gateway`
  - `backend/app/channels`
  - `frontend/src`

DeerFlow is a useful reference because it treats a long task as a whole workspace, not a single chat turn. The public material and local implementation both point to the same capability families: long-horizon tasks, subagents, sandboxed filesystem execution, progressive skills and tools, memory, message gateways, search/fetch/knowledge connectors, human-in-the-loop checkpoints, artifact editing, multimodal output, streaming APIs, and deployment/security defaults.

Nomici should learn from those capabilities without copying DeerFlow's runtime shape. Nomici remains the local-first control plane:

- Gateway owns coordination records, sessions, policy, approvals, traces, artifacts, task ledgers, redaction, and message normalization.
- Runtimes own their native reasoning loops, tools, memory, and execution engines.
- Packs and AgentSpec expose portable composition.
- Adapters connect specialized systems such as Codex, Claude Code, Hermes, OpenClaw, LangGraph, OpenAI Agents SDK, MCP servers, search providers, knowledge stores, and sandbox providers.

## Current Nomici Baseline

Nomici already has several pieces that map to the DeerFlow reference:

- Gateway, CLI, read-only Console, token auth, and SQLite-backed state.
- Provider profiles, OpenAI-compatible model test, and secret references.
- Pack installation and a `developer-team` starter pack.
- AgentSpec and AgentGraph validation with clear unsupported-edge failures.
- Single-node model-backed graph execution, `cli_agent` execution, and one-step handoff.
- Shared Context items and handoff snapshots.
- Trace events, approvals, policy service, and artifact metadata foundations.
- Run sessions, run tasks, and sandbox records in storage migrations.
- Runtime registry and local process lifecycle commands.

The missing work is not "add an agent framework inside Gateway." The missing work is making the control-plane records, APIs, policies, adapters, and Console surfaces rich enough that DeerFlow-like long tasks can be managed, observed, resumed, and governed across multiple runtimes.

## Complete Improvement Map

| DeerFlow reference capability | DeerFlow evidence | Nomici status | Required Nomici improvement | Target phase |
| --- | --- | --- | --- | --- |
| Long-horizon tasks | Threads, runs, checkpointing, run history, todo middleware, runtime journal | Run sessions and tasks exist in storage; execution is still narrow | Make sessions/tasks authoritative across CLI, Console, channels, and adapters; add durable status transitions, resume/read APIs, budgets, cancellation, and checkpoint references | Phase 1 |
| Coordinator / planner / researcher / coder / reporter | Lead agent plus dynamic subagents and built-in subagent registry | `developer-team` pack exists; graph execution is mostly single-node or linear handoff | Promote role metadata into pack manifests, represent planner/worker/reporter ownership in tasks, support bounded sequential chains before parallel fan-out | Phase 2 |
| Parallel subagents | Subagents can run with isolated context and limits | Parallel edges validate but do not execute | Add fan-out/fan-in only after task ledger, budget caps, workspace locks, and result merge contracts exist | Phase 5 |
| Sandbox and filesystem | Local sandbox, AIO Docker sandbox, workspace/uploads/outputs, read-only skills mount | Sandbox policy metadata and sandbox records exist; execution support is adapter-specific | Define provider interface and workspace lifecycle records; map uploads, workspace, outputs, artifact roots, cleanup, and provider readiness | Phase 3 |
| File operations | `read_file`, `write_file`, `str_replace`, `ls`, present-file workflow | Policy and approvals exist; tool mediation is incomplete | Route mediated file tools through Tool Broker with sandbox constraints, approvals, redaction, and artifact registration | Phase 3 |
| Bash and code execution | Bash tools and sandbox audit middleware | CLI runners exist; mediated shell tool is not complete | Add bash tool contract with risk classification, working directory policy, timeout, captured output, and approval flow | Phase 3 |
| Search and fetch | Tavily, InfoQuest, DuckDuckGo, Jina, Firecrawl, Exa, image search | Provider setup is model-focused | Add research tool providers as Tool Packs with auth metadata, request logging, redaction, rate limits, and provider doctor checks | Phase 4 |
| MCP integration | MCP config, tools, OAuth, extension config | MCP is planned, not a full mediated registry | Add MCP registry, server lifecycle/status, OAuth/secret references, tool allowlists, and per-tool policy metadata | Phase 4 |
| Progressive skills | Public/custom skills, parser, installer, security scanner, tool policy, skill management tool | Packs can install agents; skill registry is planned | Add skill metadata, triggers, risk, required tools, selected-skill trace events, explicit run-level enablement, and progressive context injection | Phase 4 |
| Tool search / deferred tools | Tool search and deferred tool filter middleware | No equivalent surface | Add searchable tool/skill catalog so runtimes receive only relevant tools/briefings | Phase 4 |
| Context engineering | Summarization middleware, dynamic context middleware, isolated subagent contexts | Shared Context exists for handoffs | Add session summaries, context-window budgeting, run-state compression, and artifact-backed intermediate summaries | Phase 5 |
| Long-term memory | Memory middleware, memory store, memory queue/updater, user context | Shared Context can store curated items | Add explicit memory scopes, promotion proposals, approval before persistence, review/delete UI, and no scraping of runtime-private memory | Phase 5 |
| Human-in-the-loop clarification | Clarification tool and middleware | Approval queue exists; clarification state is not first-class | Add `needs_clarification`, `plan_review`, and `blocked` task states with resumable user responses | Phase 6 |
| Plan mode | Todo middleware and plan-mode config | Task records exist; plan editing is not surfaced | Add plan artifacts, user edits, approval gates, and task regeneration from approved plans | Phase 6 |
| Artifact production | Artifacts router, present-file tool, outputs directory | Artifact metadata foundation exists | Add artifact revisions, review status, source task, preview metadata, and Console artifact viewer/editor | Phase 6 |
| Uploads | Upload routes, upload processing middleware, thread-scoped upload paths | No complete upload workflow | Add Gateway upload APIs, sandbox path mapping, malware/size/type policy, trace events, and adapter handoff references | Phase 6 |
| Multimodal outputs | Report, podcast/TTS, presentation, image/video skills | Artifact types are minimal | Add pack contracts for reports, slide decks, audio, transcripts, image sets, datasets, and provider-specific media adapters | Phase 7 |
| Vision/image handling | View-image tool and middleware | Model capability metadata exists | Add image artifact/input metadata, vision capability checks, safe previews, and adapter-specific image payload support | Phase 7 |
| Message gateways | Telegram, Slack, Feishu/Lark, WeCom, DingTalk channels | Message gateway is planned | Normalize inbound channel messages into sessions, route updates through channel adapters, redact outbound payloads, and keep channels disabled by default | Phase 6 |
| Streaming API | LangGraph-compatible thread/run/stream APIs with SSE modes | Gateway APIs are local and Nomici-specific | Add stable run event stream for Console and adapters; consider LangGraph-compatible adapter as an integration surface, not core identity | Phase 8 |
| Frontend workspace | Next.js workspace, chat, model selector, task/todo/artifact UI | Console is read-only overview | Add operational Console views for sessions, tasks, plan review, tools, skills, artifacts, uploads, and channel status | Phases 1-7 |
| Model configuration | LangChain model factory with capabilities | Provider profiles exist | Expand provider catalog with context/cost/tool/vision/thinking capabilities and compatibility checks per adapter | Phase 1 |
| Observability | LangSmith tracing, run events, token usage middleware | Trace store exists | Add token usage, budget accounting, external trace links, event export, and clearer redaction policy | Phase 5 |
| Guardrails and error handling | Guardrails middleware, loop detection, tool/LLM error handling | Policy engine foundation exists | Add loop detection, budget stop, repeated tool failure handling, retry policy, and guardrail result traces | Phase 5 |
| Embedded client | `DeerFlowClient` mirrors Gateway API | CLI is primary client | Add a typed local SDK only after Gateway API stabilizes; keep CLI and Console as first clients in v0.1/v0.2 | Phase 8 |
| Deployment | Docker, nginx, local dev, production compose, resource guidance | Install script and local Gateway exist | Add Docker Compose only after local control-plane flows are hardened; preserve loopback/token defaults | Phase 8 |
| Auth and public exposure | Local auth, CSRF, warnings for public deployment | Gateway token auth exists | Before server/team mode, add OIDC/RBAC/workspaces, stricter CSRF/origin guidance, and channel-specific auth review | Phase 9 |

## Phased Plan

### Phase 0: v0.1 Bootstrap Closure

Goal: finish the current local-first proof slice before adding DeerFlow-inspired breadth.

Scope:

- Keep CLI-first setup stable.
- Keep read-only Console honest about implemented and unavailable actions.
- Ensure provider setup, pack install, first run, trace, approval, and artifact basics pass on a clean machine.
- Keep Gateway bound to loopback with token auth by default.
- Update docs whenever a command is implemented or deferred.

Exit criteria:

- `make test` and `make build` pass.
- README status matches actual behavior.
- The `developer-team` pack can run the implemented path without hidden assumptions.
- Unsupported graph edges, tools, and Console actions fail clearly.

### Phase 1: Durable Run Workspace

Goal: make long tasks durable and inspectable before adding more execution paths.

Scope:

- Treat `run_sessions` and `run_tasks` as authoritative state, not derived UI state.
- Add Gateway read APIs for session summaries, task ledgers, and task details.
- Add status transitions for `queued`, `running`, `blocked`, `approval_waiting`, `failed`, `cancelled`, and `completed`.
- Add cancellation and timeout/budget fields at session and task level.
- Attach graph snapshot, context snapshot, approvals, and artifact references to each task.
- Add Console session/task views backed by APIs.

Acceptance:

- A run can be refreshed and still show the same session/task state.
- A handoff creates visible task records.
- Failed, blocked, cancelled, and approval-waiting states are durable.
- Console no longer infers task state only from trace text.

### Phase 2: Graph-Native Role Packs

Goal: turn the DeerFlow role pattern into Nomici packs without hardcoding DeerFlow roles in Gateway.

Scope:

- Extend pack manifest metadata for role purpose, required tools, default handoff mode, model preference, and context needs.
- Make `developer-team` expose coordinator, planner, researcher, coder, and reporter roles as pack-defined agents.
- Support sequential planner -> worker -> reporter chains through existing handoff execution.
- Add bounded Shared Context for each role and trace events for role ownership.
- Keep parallel/fallback branches as explicit unsupported execution until Phase 5.

Acceptance:

- A starter pack exposes a single entrypoint but renders the role path.
- Each role receives bounded context, not a dump of the whole session.
- CLI, Console, and trace show which role owned each task.

### Phase 3: Sandbox, Workspace, And Mutating Tools

Goal: provide real workspace records and a conservative tool path for files and shell.

Scope:

- Define sandbox provider types: `local`, `container`, `remote`.
- Persist workspace/uploads/outputs/artifacts roots per run and per task where needed.
- Detect Docker, Podman, and Apple Container availability.
- Add cleanup state and orphaned-workspace detection.
- Implement mediated file tools first: read, list, write, replace, present artifact.
- Implement mediated bash with timeout, working directory, output capture, and approval policy.
- Keep local execution default and container execution opt-in until adapter support is proven.

Acceptance:

- `nomici setup --sandbox container` records intent and `nomici doctor` reports executable readiness.
- Runs show workspace, uploads, outputs, artifact root, and cleanup state.
- File write and bash cannot run without approval in untrusted mode.
- Trace events redact sensitive file paths and command payloads where configured.

### Phase 4: Skills, Tools, Search, Fetch, And MCP

Goal: make extensibility visible and safe without flooding runtime context.

Scope:

- Add AgentSpec fields for project, agent, and run-level skills.
- Add pack manifest skill metadata: name, description, triggers, files, required tools, compatibility, and risk.
- Add `nomici skill list`, `nomici skill inspect`, and pack install wiring.
- Add searchable tool/skill catalog for progressive loading.
- Add read-only search/fetch tool packs for DuckDuckGo, Brave, Tavily, Searx/SearxNG, Jina, InfoQuest-style fetch, Firecrawl, Exa, and image search where providers are available.
- Add MCP registry entries with server status, auth references, OAuth metadata, and tool allowlists.

Acceptance:

- Installed skills and tools are visible in CLI, Console, and graph snapshots.
- A run can enable a skill explicitly.
- Selected skills/tools are recorded in trace.
- Read-only search/fetch emits redacted trace events.
- MCP servers are disabled or allowlisted by default unless explicitly configured.

### Phase 5: Context, Memory, Budgets, And Controlled Parallelism

Goal: support longer tasks without depending on raw conversation context or runtime-private memory.

Scope:

- Add context-window budgets and summary checkpoints.
- Add memory scopes: project, user preference, run summary, decision, artifact summary, and adapter note.
- Add explicit memory promotion proposals from completed runs.
- Require user approval before durable memory promotion.
- Add token usage, budget accounting, loop detection, repeated-tool-failure handling, and stop conditions.
- Add parallel fan-out/fan-in only after workspace locks, task budgets, result schemas, and merge contracts are in place.

Acceptance:

- A completed run can propose memory entries for review.
- User approval is required before promotion.
- Runtime-native memory is not scraped by default.
- Parallel tasks cannot write to the same workspace target without a lock or clear conflict behavior.
- Budget and loop stops are visible in trace and Console.

### Phase 6: Human-In-The-Loop, Uploads, Artifacts, And Channels

Goal: make user collaboration and non-Console entrypoints first-class.

Scope:

- Add task states for `needs_clarification`, `plan_review`, and `blocked`.
- Let users edit or approve a plan before execution continues.
- Store user edits as run messages and artifact revisions.
- Add upload APIs with size/type policy, sandbox path mapping, and trace references.
- Add Console surfaces for plan review, clarification, uploads, artifact preview, artifact revision, and task resume.
- Add channel config schema and adapters for Slack, Telegram, Feishu/Lark, WeCom, and DingTalk as optional packs/adapters.
- Normalize inbound channel messages into the same run session store used by CLI and Console.
- Redact and rate-limit outbound channel updates.

Acceptance:

- A run can pause for clarification or plan approval and resume from the approved input.
- Uploaded files are traceable and mapped into the run workspace.
- Artifact revisions preserve source task and user edit history.
- CLI, Console, and channel-created tasks share the same session/task store.
- Secrets and oversized artifacts are never sent to channels by default.

### Phase 7: Multimodal Artifact Packs

Goal: support non-text outputs through packs, not core assumptions.

Scope:

- Add artifact types for report, slide deck, audio, transcript, image set, video, dataset, and web bundle.
- Define pack contracts for research reports, presentations, TTS, podcasts, image generation, video generation, and dashboard/web generation.
- Add provider-specific media generation behind adapter/tool contracts.
- Add Console preview metadata for supported artifact types.
- Keep large binary storage local and explicit.

Acceptance:

- Artifact metadata captures type, source task, files, preview, review state, and revision lineage.
- Media packs can be installed without changing Gateway core.
- Console can show generated artifact references and safe previews.

### Phase 8: Framework Adapters, Compatible APIs, SDK, And Deployment

Goal: integrate durable external runtimes while preserving Nomici's control-plane boundary.

Scope:

- Add LangGraph adapter and optional LangGraph-compatible API surface for integration.
- Add OpenAI Agents SDK, CrewAI, Google ADK, and Agent Squad adapters after adapter contract tests exist.
- Add typed local SDK only after Gateway API contracts stabilize.
- Add Docker Compose and deployment docs after local flows are hardened.
- Add event-stream compatibility tests for CLI, Console, SDK, and adapters.

Acceptance:

- Framework adapters report capability, health, run state, and trace mapping consistently.
- Compatible API surfaces do not become the internal source of truth.
- Docker deployment preserves loopback/token defaults unless explicitly configured otherwise.

### Phase 9: Team, Server, Registry, And Marketplace

Goal: move beyond local-first single-user mode only after security and governance are ready.

Scope:

- Add Postgres, workspaces, OIDC, RBAC, audit retention, and remote workers.
- Add signed pack distribution and registry metadata.
- Add channel and public network deployment hardening.
- Add CODEOWNERS, branch protection, release signing, and supply-chain checks before wider publication.

Acceptance:

- Team/server mode has explicit auth, origin, network, secret, and audit boundaries.
- Signed packs are verified before install.
- Marketplace trust claims do not bypass permission review.

## PR Sequence

Each PR should stay reviewable and should not bypass earlier data-model work.

| PR | Title | Phase | Main deliverable |
| --- | --- | --- | --- |
| A | Run Sessions And Task Ledger APIs | 1 | Durable session/task read models and Console task view |
| B | Role Pack Metadata And Sequential Role Handoff | 2 | Pack-defined coordinator/planner/researcher/coder/reporter path |
| C | Sandbox Workspace Records And Provider Detection | 3 | Workspace/uploads/outputs/artifact roots and doctor readiness |
| D | Mediated File Tools And Bash Approval Flow | 3 | Policy-controlled file mutation and shell execution contracts |
| E | Skill Registry And Progressive Loading | 4 | Skill metadata, CLI, trace, and selected briefing injection |
| F | Tool Registry With Search/Fetch Providers | 4 | Read-only research tools with provider auth and redacted traces |
| G | MCP Registry And Allowlisted Tool Brokering | 4 | MCP server status, auth refs, OAuth metadata, policy allowlists |
| H | Context Summaries And Memory Promotion | 5 | Session summaries, promotion proposals, approval-before-memory |
| I | Budgets, Loop Detection, And Parallelism Prereqs | 5 | Token/budget accounting, stop conditions, workspace locks |
| J | Human Plan Review And Clarification States | 6 | Plan edit/approve/resume and `needs_clarification` tasks |
| K | Uploads And Artifact Revisions | 6 | Upload path mapping, artifact revision history, Console previews |
| L | Message Gateway Channel Adapters | 6 | Slack/Telegram/Feishu/Lark/WeCom/DingTalk optional adapters |
| M | Multimodal Artifact Pack Contracts | 7 | Report/slides/audio/image/video/dataset/web bundle artifact types |
| N | Framework Adapter Contract Suite | 8 | LangGraph/OpenAI Agents/CrewAI/ADK adapter test harness |
| O | Compatible Streaming API And Local SDK | 8 | Stable event stream and typed client after APIs settle |
| P | Server Mode And Signed Registry Foundations | 9 | Postgres/OIDC/RBAC/signed pack groundwork |

## Cross-Cutting Requirements

Every implementation PR in this roadmap must include:

- Storage migrations or an explicit reason none are needed.
- Trace events for new state transitions.
- Policy and redaction behavior for side effects.
- CLI tests for setup, doctor, and introspection paths.
- Gateway handler tests for new APIs.
- Console behavior that degrades clearly when a capability is configured but unavailable.
- Documentation updates and security notes.
- Compatibility notes for adapters and packs touched by the change.

## Guardrails

- Do not make Gateway run unbounded autonomous loops.
- Do not run parallel agents until task ledger, workspace locking, and budget controls exist.
- Do not treat message streams as the source of truth.
- Do not store raw API keys, channel tokens, private runtime memory, or unredacted tool payloads.
- Do not expose public network channels without explicit auth, origin, and deployment guidance.
- Do not turn DeerFlow-specific role names into hardcoded Gateway concepts.
- Do not block local-first v0.1 on Docker, Kubernetes, marketplace, or team/server mode.

## Immediate Next PR

The next implementation PR after this planning update should be PR A: Run Sessions And Task Ledger APIs.

Recommended first slice:

- Add `GET /api/sessions` and `GET /api/sessions/{id}/tasks`.
- Add CLI read commands for sessions and tasks.
- Update Console to render durable task records.
- Backfill current one-step runs into a session plus root task.
- Add tests for refresh, failed state, approval-waiting state, and handoff task creation.
