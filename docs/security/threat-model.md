# Threat Model

This document describes the main security threats Nomici Orchestrator should consider before implementation.

Nomici is a control plane. It can manage local processes, route agent calls, invoke tools, expose Gateway APIs, and store traces. That makes conservative defaults mandatory.

## Assets

Assets to protect:

- Gateway token
- API keys and secret references
- Local files and workspaces
- Runtime process credentials
- Agent prompts and responses
- Trace and audit logs
- Approval decisions
- Install and update path
- Release artifacts
- User machine and local network

## Trust Boundaries

Primary boundaries:

- User and Nomici CLI
- Browser and Nomici Gateway
- Gateway and local runtimes
- Gateway and external agent endpoints
- Gateway and MCP servers
- Gateway and remote A2A agents
- Gateway and cloud model providers
- Install script and release artifacts

## Threats

### Gateway Token Disclosure

Risk:

An attacker with a Gateway token can call control-plane APIs. If `/v1/*` endpoints are enabled, the attacker may be able to run agents and indirectly reach sensitive tools.

Mitigations:

- Generate strong tokens.
- Store tokens in keychain or strict-permission files.
- Redact tokens from logs, traces, and debug bundles.
- Keep Gateway on loopback by default.
- Treat `/v1/*` as operator-level access unless scoped tokens are implemented.

### Public Gateway Exposure

Risk:

Binding Gateway to `0.0.0.0` or exposing it through a public proxy can allow remote control of local agents and tools.

Mitigations:

- Bind to `127.0.0.1` by default.
- Require explicit remote-access config.
- Require auth for non-loopback binds.
- Recommend TLS or trusted private networking.
- Audit remote-access enablement.

### Malicious MCP Server

Risk:

An MCP server can expose tools that read files, write files, execute commands, call APIs, or exfiltrate data through tool outputs.

Mitigations:

- Treat MCP servers as untrusted by default.
- Require approval for high-risk tools.
- Scope filesystem and network permissions.
- Log tool calls and policy decisions.
- Show risk summaries in approval UI.

### Malicious Remote A2A Agent

Risk:

A remote agent can request sensitive context, return malicious instructions, or impersonate capabilities.

Mitigations:

- Treat remote A2A agents as untrusted by default.
- Do not share secrets by default.
- Trace all calls.
- Require explicit trust configuration.
- Keep capability manifests as claims, not proof.

### External Agent Endpoint Overreach

Risk:

External endpoints such as Hermes or OpenClaw may have powerful tool access. Routing through them can execute actions outside Nomici's direct control.

Mitigations:

- Mark external agent endpoints as operator surfaces.
- Require explicit endpoint configuration.
- Apply Nomici policy before Nomici-mediated calls.
- Document that external runtime policies still matter.
- Preserve audit events for delegated calls.

### Runtime Process Escape or Misconfiguration

Risk:

Local runtimes may run with broad filesystem or network access. A compromised or misconfigured runtime can affect the host.

Mitigations:

- Use separate workspaces where possible.
- Track runtime commands, env refs, ports, and logs.
- Require approval for untrusted runtime starts where appropriate.
- Add Docker or OS sandbox support later.
- Warn when runtime permissions are broad.

### Install Script Tampering

Risk:

Install scripts can become a remote code execution path.

Mitigations:

- Avoid `sudo` by default.
- Verify release checksums.
- Add signature verification when release signing exists.
- Avoid overwriting config.
- Support uninstall.
- Keep install logic small and auditable.

### Debug Bundle Leakage

Risk:

Debug bundles may include secrets, prompts, file paths, tokens, traces, or runtime logs.

Mitigations:

- Redact known secret patterns.
- Exclude full `.env` files.
- Exclude raw Authorization headers.
- Document that users should review bundles before sharing.
- Add tests for redaction.

### Trace and Log Leakage

Risk:

Traces and logs can contain prompts, responses, tool arguments, file paths, and provider metadata.

Mitigations:

- Store traces locally by default.
- Redact secrets in exports.
- Do not enable telemetry by default.
- Make trace export explicit.
- Add retention controls later.

### Dependency or CI Compromise

Risk:

A compromised dependency or GitHub Action can alter builds, steal secrets, or publish malicious releases.

Mitigations:

- Use least-privilege workflow permissions.
- Avoid secrets in PR workflows from forks.
- Review dependency updates.
- Pin release-sensitive actions before stable release.
- Publish checksums and provenance.

## Out of Scope for v0.1

v0.1 does not claim to:

- Secure arbitrary malicious local code.
- Provide hard sandboxing for all runtimes.
- Provide multi-user RBAC.
- Provide tenant isolation.
- Make public internet exposure safe by default.
- Guarantee deterministic re-execution of external agent runs.

## Review Cadence

Review this threat model when:

- Gateway auth changes.
- `/v1/*` endpoint behavior changes.
- MCP or A2A behavior changes.
- Runtime execution behavior changes.
- Install or release scripts are added.
- Remote access is added.
- Debug bundle or trace export behavior changes.
