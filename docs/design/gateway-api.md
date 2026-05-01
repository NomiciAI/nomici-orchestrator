# Gateway API Design

## Purpose

Nomici Gateway is the single control-plane API.

CLI and Console should use the same Gateway APIs once Gateway is running. Bootstrap commands such as `nomici init`, `nomici spec validate`, and `nomici gateway run` may run without Gateway.

## API Principles

- REST for command/query operations.
- WebSocket or SSE for events.
- JSON request and response bodies.
- Stable error envelope.
- Token auth by default.
- No raw secrets in responses.
- Version APIs conservatively.

## Response Envelope

Successful response:

```json
{
  "data": {},
  "warnings": [],
  "request_id": "req_01H"
}
```

Error response:

```json
{
  "error": {
    "code": "missing_reference",
    "message": "Model gpt is not configured.",
    "remediation": "Run nomici model setup or update nomici.yaml.",
    "details": {}
  },
  "request_id": "req_01H"
}
```

Error codes should be stable and machine-readable.

## Auth

Default:

- Gateway binds to `127.0.0.1`.
- Bearer token auth is required.
- CLI obtains token from profile config or keychain.
- Console uses same-origin token/session flow.

Auth errors:

- `auth_missing`
- `auth_invalid`
- `auth_expired`
- `auth_forbidden`

## API Groups

```text
/api/health
/api/setup
/api/providers
/api/models
/api/packs
/api/graphs
/api/agents
/api/runtimes
/api/tools
/api/runs
/api/tasks
/api/traces
/api/approvals
/api/artifacts
/api/policies
/api/secrets
```

Protocol surfaces:

```text
/v1/*
/a2a/*
/mcp/*
/events
/ws
```

Reserved paths should not imply full compatibility.

## Event Stream

Event stream messages:

```json
{
  "event_id": "evt_01H",
  "type": "runtime.health_changed",
  "time": "2026-05-01T00:00:00Z",
  "data": {}
}
```

Use cases:

- Console live status
- trace streaming
- approval updates
- runtime log tail metadata

v0.1 can start with SSE for server-to-client events and add WebSocket later.

## API Versioning

v0.1 internal API can be unstable while private.

Before public release:

- document supported routes
- add `/api/version`
- avoid breaking CLI/Console mismatch
- prefer additive changes

## Tests

- route existence
- auth required
- error envelope
- no secret leakage
- CLI and Console client fixtures
