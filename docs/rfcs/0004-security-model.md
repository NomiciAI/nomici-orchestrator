# RFC 0004: Security Model

Status: Draft
Date: 2026-04-30
Target release: Nomici Orchestrator v0.1

## Summary

Nomici Orchestrator is a control plane. A compromised Gateway, token, adapter, MCP server, or external agent can become a path to powerful tools. Security must be a first-version design constraint, not a later enterprise feature.

v0.1 should be local-first, token-protected, conservative by default, explicit about trust boundaries, and approval-first for high-risk actions.

## Security Goals

Nomici v0.1 must:

- Bind Gateway to `127.0.0.1` by default.
- Generate a strong local Gateway token by default.
- Keep remote access disabled by default.
- Never write raw secrets to `nomici.yaml`.
- Redact secrets from logs, traces, debug bundles, and UI responses.
- Treat MCP servers as untrusted by default.
- Treat remote A2A agents as untrusted by default.
- Treat external OpenAI-compatible agent endpoints as operator-level surfaces unless scoped otherwise.
- Require approval for high-risk actions by default.
- Write policy decisions and approval outcomes to the audit log.
- Keep Web UI code from directly receiving raw secret values.

## Non-Goals

v0.1 does not need:

- Multi-user RBAC.
- OIDC.
- SAML.
- Organization tenancy.
- Public internet Gateway exposure.
- Fine-grained per-user API tokens.
- Enterprise policy language.
- Full sandbox escape prevention for arbitrary local processes.

These should be added later without weakening v0.1 defaults.

## Trust Zones

### Trusted Local Operator

The person using the local CLI and Web Console on the machine where Gateway runs.

Assumptions:

- This user can approve high-risk actions.
- This user can access local files and environment variables.
- This user owns the workspace.

### Nomici Gateway

The local control-plane process.

Responsibilities:

- Authenticate clients.
- Enforce policy.
- Broker runtime operations.
- Store traces and audit logs.
- Redact secrets.
- Serve the Web Console.

If Gateway is compromised, the local workspace and managed runtimes should be considered compromised.

### Nomici Console

The browser UI served by Gateway.

Rules:

- Must not receive raw secrets.
- Must authenticate to Gateway.
- Should use same-origin Gateway APIs by default.
- Should not be embeddable by untrusted origins.
- Should avoid storing long-lived secrets in localStorage.

### Local Runtimes

Managed processes such as Hermes, OpenClaw, Ollama, vLLM, and custom workers.

Rules:

- Each runtime should have a workspace scope.
- Each runtime should have a permission scope.
- Runtime logs may contain secrets and must be redacted before display or export where practical.
- Runtime process commands must be visible in inspect/debug output with secret values redacted.

### External Agents

External agent endpoints, including Hermes API servers, OpenClaw Gateways, and remote A2A agents.

Rules:

- External agents are untrusted by default.
- External agent capabilities are claims, not proof.
- Calls to external agents must be traceable.
- External agents must not receive secrets unless explicitly configured.

### MCP Servers

MCP servers provide tools, data, and workflow capabilities.

Rules:

- MCP servers are untrusted by default.
- Tool calls from untrusted servers require policy evaluation.
- Filesystem, shell, email, browser, calendar, deployment, and unknown network actions are high risk.

## Default Gateway Security

Default config:

```yaml
deployment:
  gateway:
    bind: 127.0.0.1
    port: 8787
    auth:
      mode: token
    remote_access:
      enabled: false
    openai_compatible:
      enabled: false
```

Rules:

- `0.0.0.0` bind must require explicit config.
- Remote access must require explicit config.
- Remote access should require TLS or a trusted proxy.
- Token auth is required unless explicitly disabled for development.
- Disabling auth must require a loud warning and should be blocked for non-loopback bind.

## OpenAI-Compatible Endpoint Boundary

Nomici may expose:

- `/v1/models`
- `/v1/chat/completions`
- `/v1/responses`
- `/v1/embeddings`

Security rule:

> A valid token for Nomici's OpenAI-compatible endpoint must be treated as an operator credential unless a future scoped-token system says otherwise.

This endpoint may route to agents with tools. If those agents can invoke sensitive tools, a caller with endpoint access may indirectly reach those tools. Therefore:

- Keep `/v1/*` disabled by default or clearly operator-scoped.
- Do not expose `/v1/*` directly to the public internet.
- Apply the same policy engine used by normal Gateway runs.
- Log all `/v1/*` run events.
- Include endpoint calls in audit logs.

## Secrets

Allowed secret references:

```yaml
api_key_env: OPENAI_API_KEY
```

Allowed later:

```yaml
secret_ref: keychain://nomici/openai
secret_ref: op://Engineering/OpenAI/api_key
secret_ref: bitwarden://item/openai-api-key
```

Forbidden:

```yaml
api_key: sk-...
token: secret-value
password: plaintext-password
```

Rules:

- `nomici.yaml` must not contain raw secrets.
- `.env` files are local and must not be exported in debug bundles.
- Web Console displays secret presence and source, not value.
- CLI output redacts secret-looking values.
- Trace export redacts request headers and known secret fields.
- Debug bundle redacts environment variables matching secret patterns.

## Approval Model

Policy decisions:

- `allow`
- `deny`
- `approval`

Approval scopes:

- once
- for run
- for session
- for workspace, future
- always, future and dangerous

High-risk actions requiring approval by default:

- Shell execution
- Filesystem write
- Email send
- Calendar mutation
- Deployment
- Git push
- Pull request creation
- Browser automation with side effects
- HTTP request to unknown host
- MCP tool from untrusted server
- OpenClaw tool invoke
- Runtime process start with untrusted command

Approval records must include:

- Approval ID
- Run ID
- Agent ID
- Runtime ID if applicable
- Tool or operation
- Arguments summary
- Risk explanation
- Requested time
- Decision
- Deciding actor
- Decision time

The UI should show a diff preview where applicable, especially for filesystem writes, patches, git operations, and generated messages.

## Policy Defaults

Default policy:

```yaml
policies:
  shell:
    mode: approval

  filesystem:
    read:
      - .
    write: approval

  network:
    allow:
      - localhost
      - 127.0.0.1
    unknown_host: approval

  email:
    send: approval

  calendar:
    mutate: approval

  mcp:
    default_trust: untrusted
    untrusted_tool_call: approval

  a2a:
    default_trust: untrusted
```

Implementation can start with a simpler internal policy representation as long as behavior matches these defaults.

## Audit Log

Audit events:

- `auth.succeeded`
- `auth.failed`
- `runtime.started`
- `runtime.stopped`
- `tool.requested`
- `tool.completed`
- `tool.failed`
- `approval.requested`
- `approval.granted`
- `approval.denied`
- `policy.allowed`
- `policy.denied`
- `policy.approval_required`
- `secret.resolved`
- `gateway.token.rotated`
- `remote_access.enabled`

Rules:

- Audit log is append-only in normal operation.
- Export must redact secrets.
- Audit events should include correlation IDs for run, agent, runtime, and request.

## Remote Access

Remote access is off by default.

Supported future modes:

- Trusted reverse proxy with TLS
- Tailscale or private tailnet
- SSH tunnel
- mTLS

Remote access requirements:

- Explicit bind configuration.
- Auth enabled.
- Clear warning in CLI.
- Audit event when enabled.
- Recommended TLS or trusted proxy.

Plain HTTP on LAN should be treated as unsafe unless explicitly enabled.

## Runtime Isolation

v0.1 local process isolation is limited.

Nomici should still provide:

- Separate workspaces per runtime where configured.
- Separate environment scopes.
- Separate logs.
- Separate health checks.
- Permission metadata per runtime.
- Clear warnings when running broad-permission processes.

Later versions can add:

- Docker sandboxing
- Firecracker or microVM isolation
- OS sandbox profiles
- Kubernetes namespaces

## Debug Bundle

`nomici debug bundle` should include:

- Nomici version
- OS and architecture
- Gateway config with secrets redacted
- AgentSpec with secrets redacted
- Runtime status
- Recent health checks
- Recent failed traces
- Recent Gateway logs
- Recent runtime logs after redaction

It must not include:

- Raw API keys
- Raw bearer tokens
- Full `.env` files
- Private SSH keys
- Browser cookies
- Unredacted Authorization headers

## Security Documentation Requirements

Docs must clearly explain:

- Gateway token is powerful.
- OpenAI-compatible endpoint is an operator surface.
- MCP servers are untrusted by default.
- External agents are untrusted by default.
- Shell and filesystem writes can modify the user's machine.
- Remote access should use TLS or trusted private networking.
- Debug bundles are redacted but should still be reviewed before sharing.

## Open Questions

- Should `/v1/*` be disabled by default? Recommendation: yes for v0.1.
- Should local loopback Web Console require token entry? Recommendation: yes, but make first-run pairing smooth.
- Should CLI commands bypass Gateway auth on the same machine? Recommendation: no; use the same token path where possible.
- Should Nomici store Gateway token in OS keychain by default? Recommendation: yes where available, file fallback with strict permissions.
- Should policy be implemented with a simple rule evaluator first or embedded OPA/Rego? Recommendation: simple evaluator first.
