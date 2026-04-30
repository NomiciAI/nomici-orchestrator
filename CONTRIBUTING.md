# Contributing to Nomici Orchestrator

Thanks for your interest in contributing. Nomici is currently in the RFC and early implementation phase, so design clarity matters as much as code.

## Project Status

Nomici is not yet a released product. The current repository contains RFCs and foundation documents. Early pull requests should keep scope small and align with the RFCs in `docs/rfcs/`.

## Development Principles

- Keep the end-user command surface centered on `nomici`.
- Keep contributor commands centered on `make`.
- Prefer local-first behavior.
- Treat Gateway, `/v1/*`, MCP, A2A, shell, filesystem, and runtime management as security-sensitive surfaces.
- Do not add cloud-first assumptions to v0.1.
- Do not embed secrets in examples, tests, traces, or docs.

## Tooling

The intended contributor command surface is:

```bash
make dev
make build
make test
make lint
make fmt
make clean
```

These commands may be placeholders until the engineering scaffold is added. When implementation begins, CI and docs should use these commands rather than raw `go` or `pnpm` commands where possible.

Expected local tools after scaffold:

- Go
- Node.js
- pnpm
- make

End users should not need these tools after installing a release binary.

## RFC Process

Use an RFC when a change affects:

- Product positioning or v0.1 scope
- AgentSpec
- Gateway API shape
- Security model
- Runtime or adapter contract
- Release or install behavior
- Multi-user or remote access behavior

RFC lifecycle:

1. Draft a document in `docs/rfcs/`.
2. Discuss tradeoffs in the pull request.
3. Update the RFC until the decision is clear.
4. Maintainers mark it accepted, amended, or superseded in a later change.

## Pull Requests

Before opening a PR:

- Keep changes focused.
- Add tests for behavior changes when test infrastructure exists.
- Update docs for user-visible changes.
- Redact secrets from logs and examples.
- Call out security-sensitive behavior in the PR description.

Preferred validation:

```bash
make fmt
make lint
make test
make build
```

## Commit Signoff

Nomici expects to use Developer Certificate of Origin (DCO) rather than a Contributor License Agreement (CLA) unless a future governance decision changes this.

When DCO enforcement is enabled, sign commits with:

```bash
git commit -s
```

This adds a `Signed-off-by` line confirming that you have the right to submit the contribution.

## Security-Sensitive Contributions

Open a normal PR only if the change does not reveal an active vulnerability.

For suspected vulnerabilities, follow `SECURITY.md` and report privately.

Security-sensitive areas include:

- Gateway auth
- Gateway tokens
- Secrets and redaction
- Runtime process execution
- Shell and filesystem tools
- MCP tools
- A2A remote agents
- OpenAI-compatible `/v1/*` endpoints
- Install and update scripts
- Debug bundles

## Documentation Style

- Be direct and precise.
- Mark planned commands as planned if they are not implemented.
- Avoid implying official partnerships with external projects unless one exists.
- Distinguish "compatible with" from "endorsed by."
- Prefer examples that are safe by default.
