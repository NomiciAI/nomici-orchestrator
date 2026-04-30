# Roadmap

This roadmap describes intended direction. It is not a release promise.

## v0.1: Local Agent Control Plane

Goal:

Prove the local-first control-plane loop from `nomici.yaml` to Gateway, Console, runtime registry, agent run, traces, approvals, and logs.

Planned:

- `nomici` CLI
- Nomici Gateway
- Basic Nomici Console
- AgentSpec v0.1
- JSON Schema validation
- Model registry
- Runtime registry
- Local process runner
- OpenAI-compatible endpoint adapter
- Hermes endpoint adapter
- OpenClaw endpoint adapter
- Ollama and vLLM through provider or OpenAI-compatible configuration
- SQLite trace store
- Approval queue
- Conservative default policy
- Demo templates
- Install script

## v0.2: Protocol-First Orchestration

Planned:

- A2A server and client
- MCP registry
- A2A sidecars
- Better router
- Parallel fan-out and fan-in
- Timeline replay improvements
- Policy engine v1

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
- Full deterministic re-execution of external LLM runs
- Public internet Gateway exposure by default
- Running arbitrary untrusted code as a hard security boundary
