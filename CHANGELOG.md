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

### Changed

- Nothing yet.

### Deprecated

- Nothing yet.

### Removed

- Nothing yet.

### Fixed

- Gate 0 scaffold baseline now builds, tests, lints, prints version, and serves Gateway health.

### Security

- Security model documented before implementation begins.
