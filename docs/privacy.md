# Privacy

Nomici Orchestrator is designed as a local-first control plane.

## Default Policy

v0.1 should not include telemetry by default.

By default, Nomici should not send:

- Prompts
- Responses
- Agent traces
- Tool arguments
- File paths
- Environment variables
- API keys
- Gateway tokens
- Runtime logs
- Machine identifiers
- Usage metrics

to Nomici, nomici.ai, or any third-party analytics provider.

## Local Data

Nomici may store local operational data:

- Workspace state in `.nomici/`
- Global profile state in `~/.nomici/`
- SQLite run and trace data
- Runtime logs
- Approval history
- Gateway config
- Secret references

Secrets should be referenced, not stored directly in `nomici.yaml`.

## External Services

Nomici can call external services only when the user configures them or invokes functionality that requires them. Examples:

- Cloud LLM providers
- OpenAI-compatible endpoints
- Remote A2A agents
- Remote MCP servers
- Adapter registries, if added later
- Update and release endpoints, if added later

Those services may receive prompts, tool outputs, or metadata depending on the user's configuration and run behavior.

## Future Telemetry

If telemetry is added later, it must be:

- Opt-in by default.
- Documented clearly.
- Easy to disable.
- Redacted before transmission.
- Visible in config and CLI output.
- Covered by an RFC before implementation.

Telemetry must not include prompts, responses, secrets, trace payloads, file contents, or private runtime logs unless a user explicitly exports and shares them.

## Debug Bundles

`nomici debug bundle` should redact secrets automatically, but users should still review bundles before sharing them publicly.

Debug bundles must not include:

- Raw API keys
- Gateway bearer tokens
- Private SSH keys
- Browser cookies
- Full `.env` files
- Unredacted Authorization headers
