# Governance

## Project Model

Nomici Orchestrator is an open-source project governed by maintainers. During the early phase, maintainers prioritize product clarity, security defaults, and a small v0.1 surface over broad feature intake.

## Maintainers

Maintainers are responsible for:

- Reviewing and merging pull requests.
- Accepting or rejecting RFCs.
- Cutting releases.
- Managing security reports.
- Enforcing the Code of Conduct.
- Protecting the project scope.

The initial maintainer set will be documented once the GitHub organization is configured.

## Decision Making

Most decisions can be made in pull requests and issues.

Use an RFC for decisions that affect:

- Product scope
- Architecture
- AgentSpec
- Gateway API
- Security model
- Runtime and adapter contracts
- Release and install strategy
- Governance

Maintainers should prefer explicit written decisions over implicit behavior in code.

## RFC Status

RFCs may have these statuses:

- Draft
- Accepted
- Superseded
- Rejected

Draft RFCs describe current thinking. Accepted RFCs guide implementation. Superseded RFCs remain in history but no longer describe the current decision.

## Release Policy

Before v0.1:

- Releases may be experimental.
- APIs may change.
- Documentation must clearly mark unreleased or planned behavior.

After v0.1:

- Use semantic versioning.
- Maintain a changelog.
- Document breaking changes.
- Sign release artifacts when release infrastructure is ready.

## Security Governance

Security reports are handled privately first. Maintainers should:

- Acknowledge valid reports quickly.
- Avoid public details until a fix or mitigation exists.
- Credit reporters when requested and appropriate.
- Treat Gateway auth, secrets, runtime execution, MCP tools, A2A, and install scripts as high-risk areas.

## Scope Control

Nomici should not become a replacement for every agent framework it integrates with.

Maintainers should challenge changes that:

- Rebuild Hermes, OpenClaw, LangGraph, CrewAI, or OpenAI Agents SDK inside Nomici.
- Make cloud deployment the default before local-first workflows work.
- Add remote access without security design.
- Add broad dependencies without clear need.
- Expand v0.1 beyond the Local Agent Control Plane goal.
