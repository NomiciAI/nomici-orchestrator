# Roadmap

This roadmap describes intended direction. It is not a release promise.

## v0.1: Local Agent Control Plane

Goal:

Prove the local-first control-plane loop from provider setup and pack install to Gateway, runtime registry, first run, traces, approvals, and artifacts.

Planned:

- `nomici` CLI
- Nomici Gateway
- Basic Nomici Console
- First-run setup flow
- Provider catalog
- Provider setup wizard
- Generic OpenAI-compatible provider
- Ollama provider setup
- Model test prompt
- AgentSpec v0.1
- JSON Schema validation
- Pack manifest schema
- `ai-application-pm` pack
- `developer-team` pack scaffold
- Model registry
- Runtime registry
- Local process runner
- OpenAI-compatible endpoint adapter
- Hermes endpoint adapter
- OpenClaw endpoint adapter
- Ollama and vLLM through provider or OpenAI-compatible configuration
- SQLite trace store
- Approval queue
- Artifact metadata
- Conservative default policy
- Demo templates
- Install script

Implementation phases:

- Phase 0: Foundation
- Phase 1: Provider Setup
- Phase 2: Packs and AgentGraph
- Phase 3: Runtime and Adapters
- Phase 4: Runs, Trace, Approval, Artifacts
- Phase 5: Console MVP
- Phase 6: Pre-Public Hardening

See `docs/development-plan.md`.

## v0.2: Protocol-First Orchestration

Planned:

- Run sessions and durable task ledger
- Pack-defined role metadata and graph-native subagent roles
- Sandbox provider interface
- Skill registry and progressive loading
- Tool registry and default research/fetch tools
- Knowledge and memory bridge
- Message Gateway channel adapters
- Human-in-the-loop planning and artifact editing
- Uploads, artifact revisions, and workspace path mapping
- Context summarization, budget accounting, and loop detection
- MCP registry with allowlisted tool brokering
- A2A server and client
- A2A sidecars
- Better router
- Parallel fan-out and fan-in
- Timeline replay improvements
- Policy engine v1

See `docs/design/long-horizon-capability-roadmap.md` for the long-horizon capability gap analysis, phased delivery plan, per-PR sequence, and acceptance criteria.

## v0.3: Framework Adapters

Planned:

- LangGraph adapter
- OpenAI Agents SDK adapter
- CrewAI adapter
- Google ADK adapter
- Agent Squad adapter
- Adapter contract test suite

## v0.4: Team and Server Deployment

Planned:

- Postgres
- Multi-user support
- OIDC
- RBAC
- Workspace sharing
- Remote workers
- Docker Compose
- Helm chart

## v0.5: Registry and Marketplace

Planned:

- Agent template registry
- Adapter registry
- MCP server registry
- Signed packages
- Community templates

## Explicitly Deferred

- Model training
- Cloud-first hosted control plane
- Remote/team/multi-user mode in v0.1
- Full deterministic re-execution of external LLM runs
- Public internet Gateway exposure by default
- Running arbitrary untrusted code as a hard security boundary
- Full Office pack
- Browser automation pack
- Remote pack registry
- Signed pack distribution
