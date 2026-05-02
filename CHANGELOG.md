# Changelog

All notable changes to Nomici Orchestrator will be documented in this file.

The project follows semantic versioning once the first public release is cut.

## Unreleased

### Added

- Initial RFCs for product scope, architecture, AgentSpec v0.1, security model, and technology stack.
- Initial open-source project foundation files.
- Authoritative architecture summary and development plan.
- RFC index summarizing current design documents.
- Implementation-oriented design deep dives for provider setup, packs, graph compilation, Gateway API, runtime reconciliation, runs/traces, policy/tool brokering, and storage.
- v0.1 boundary clarifications for `gateway_agent`, scoped delivery tiers, IR convergence, Gateway health responses, pack trust roots, and Console bootstrap.
- Generic CLI agent runtime strategy for Claude Code, Codex, opencode, Aider, custom commands, editor-native agents with automation surfaces, and developer-team pack integration.
- Shared Context Layer strategy for agent-native memory boundaries, handoff briefings, and safe long-running autonomy.
- Shared Context design for context item kinds, lifecycle, handoff snapshots, adapter injection, promotion, storage, and API shape.
- Gateway agent loop boundary as a single-run coordinator rather than a durable or self-directed agent runtime.
- CLI Agent Runner process contract for prompt/briefing injection, stdout/stderr artifacts, diff capture, cancellation, and workspace locks.
- v0.1 approval scope policy: `once` and `run` only, with session/global scopes deferred.
- Living v0.1 implementation plan with implementation gates, exit commands, recording rules, and cut lines.
- Proof slice code plan for Gates 1 and 2, kept under implementation docs instead of design docs.
- SQLite-backed provider profile store, secret resolver, trace store, and OpenAI-compatible adapter.
- Gateway `/api/models/test` endpoint with response envelope, trace events, and secret redaction.
- CLI commands for `model setup`, `model list`, `model doctor`, `model test`, `run model`, `trace list`, and `trace show`.
- Minimal AgentSpec parser/validator, internal AgentGraph snapshot compiler/store, `spec validate`, `graph validate`, `graph export`, and single-node `nomici run <entrypoint>`.
- Generic `cli_agent` runner with template rendering, env refs, workspace locks, stdout/stderr artifacts, manifest/Git diff capture, trace events, `runtime inspect`, and `agent run`.
- Shared Context SQLite storage, snapshot redaction, `context list`, CLI runner `shared_context` briefing injection, structured `context_snapshot` candidates, fallback run snapshots, and one-step `cli_agent` handoff execution.
- Policy and approval v0.1 for mutable `cli_agent` execution, including approval storage, `once` and `run` scopes, `policy check`, `approvals list/grant/deny`, critical workspace denial, and trace events.
- Bundled `developer-team` pack with `pack list`, `pack inspect`, `pack install`, model-profile selection, clean AgentSpec generation, and a runnable `product_pm` entrypoint.
- Read-only Console MVP backed by Gateway APIs for overview, models, packs, latest graph, runtimes, runs, and approvals.
- GitHub Actions CI baseline with read-only workflow permissions.
- CI secret-pattern scan for docs, workflows, and source paths.
- Default Gateway bearer-token auth for non-health API endpoints, with local token file creation, CLI token forwarding, Console token entry, and `gateway token show/rotate`.
- Minimal local lifecycle commands: `up`, `down`, `ps`, and `logs` for Gateway and `local_process` runtimes.
- Top-level `doctor` and Gateway `start`, `stop`, `status`, `logs`, and `open` commands.
- `scripts/install.sh` with no-sudo defaults, source install, future release download, checksum verification, backup, custom install dir, version selection, and uninstall.
- Minimal OpenAI-compatible Gateway routes: `GET /v1/models` and non-streaming `POST /v1/chat/completions`.
- Quickstart documentation and a no-API-key `examples/basic-local-agent` demo.

### Changed

- Split the alpha bootstrap run command implementation into smaller files for graph execution, external CLI agents, policy, shared context, and display helpers.
- Marked planned README commands explicitly so the bootstrap path does not overstate `nomici up`, `nomici doctor`, or `nomici gateway open`.
- Documented `NOMICI_GATEWAY_URL`, `NOMICI_GATEWAY_TOKEN`, and local token-file behavior for alpha bootstrap.
- AgentSpec and internal AgentGraph now accept `local_process` runtimes with `start` commands for managed long-running processes.
- Source install now checks build tools and activates `pnpm` through Corepack when needed, so Quickstart users do not need to run Corepack commands manually.
- Console styling now follows the Nomici web brand language with the official solid logo, a dark default, neutral glass surfaces, and a persisted light/dark mode toggle.

### Deprecated

- Nothing yet.

### Removed

- Nothing yet.

### Fixed

- Gate 0 scaffold baseline now builds, tests, lints, prints version, and serves Gateway health.

### Security

- Security model documented before implementation begins.
- Console model APIs redact suspicious `api_key_env` values before sending data to the browser.
- Policy approval grant matching documents its current workspace-scoped fingerprint semantics.
- Gateway token auth now protects read-only Console APIs, model tests, graph/runs/runtimes/packs/approvals APIs, while `/api/health` remains public.
- Local state and Gateway token directories are created with owner-only permissions.
