# Security Policy

Nomici Orchestrator is a control plane for agent runtimes, tools, local processes, and model endpoints. Security reports are taken seriously, especially when they affect Gateway auth, secrets, runtime execution, MCP tools, A2A, install scripts, or trace redaction.

## Supported Versions

Nomici is currently in the RFC and early implementation phase. There are no supported stable releases yet.

Once releases begin, this section will list supported versions and security update policy.

## Reporting a Vulnerability

Please report suspected vulnerabilities privately.

Preferred reporting path:

- GitHub Security Advisory for `NomiciAI/nomici-orchestrator`, once the repository is public.

If that is unavailable, contact the maintainers privately through the security contact listed in the GitHub organization or project website.

Do not open a public issue for an active vulnerability.

## What to Include

Please include:

- Affected component.
- Version, commit, or branch.
- Reproduction steps.
- Impact.
- Whether secrets, local files, remote access, or tool execution are involved.
- Any logs or traces with secrets redacted.

Do not include:

- API keys.
- Bearer tokens.
- Private prompts.
- Private traces.
- Customer data.
- Exploit code beyond what is needed to demonstrate impact.

## Security-Sensitive Areas

Security-sensitive areas include:

- Gateway authentication and tokens
- OpenAI-compatible `/v1/*` endpoints
- Runtime process start, stop, and logs
- Shell execution
- Filesystem read and write
- MCP tools and servers
- A2A remote agents
- External agent endpoints
- Secrets management
- Trace export
- Debug bundles
- Install, update, and uninstall scripts

## Default Security Model

Nomici should be conservative by default:

- Gateway binds to `127.0.0.1`.
- Gateway token auth is enabled.
- Remote access is disabled.
- Secrets are referenced, not stored in `nomici.yaml`.
- MCP servers are untrusted.
- Remote A2A agents are untrusted.
- Shell, filesystem write, email, deploy, and unknown network actions require approval.
- Debug bundles redact secrets.

See `docs/rfcs/0004-security-model.md` for the detailed model.

Additional security documents:

- `docs/security/threat-model.md`
- `docs/security/dangerous-changes.md`
- `docs/security/supply-chain.md`
- `docs/privacy.md`

## OpenAI-Compatible Endpoint Warning

Nomici may expose OpenAI-compatible endpoints such as `/v1/chat/completions` and `/v1/responses`.

Treat access to these endpoints as operator-level access unless a future scoped-token system says otherwise. If a target agent can use sensitive tools, a caller with endpoint access may indirectly reach those tools through the Gateway policy path.

Do not expose these endpoints directly to the public internet.

## Research Rules

Good-faith security research is welcome when it:

- Avoids public disclosure before maintainers have time to respond.
- Avoids destructive actions.
- Avoids accessing data that is not yours.
- Uses minimal proof-of-concept steps.
- Respects rate limits and service availability.

Maintainers may ask for more detail, propose mitigations, or coordinate disclosure timing.
