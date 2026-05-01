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

### Changed

- Nothing yet.

### Deprecated

- Nothing yet.

### Removed

- Nothing yet.

### Fixed

- Nothing yet.

### Security

- Security model documented before implementation begins.
