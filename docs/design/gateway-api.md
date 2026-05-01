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

Most command/query APIs should use the standard response envelope.

`GET /api/health` is the deliberate exception. It may return a minimal naked JSON response for process supervisors, health probes, curl smoke tests, and release checks.

Future human/API status routes such as `/api/status` should use the standard envelope.

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
- CLI obtains token from `NOMICI_GATEWAY_TOKEN` or the local token file next to the state database.
- Console is served same-origin and prompts the user for the local token before calling protected APIs.

Auth errors:

- `unauthorized`
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
/api/context
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

`/api/health` is for liveness/readiness only. It should not become the general system status API.

Implemented private-bootstrap OpenAI-compatible routes:

```text
GET  /v1/models
POST /v1/chat/completions
```

These routes require the Gateway token and are operator-level surfaces. Streaming chat completions, `/v1/responses`, `/v1/embeddings`, scoped tokens, and agent-name routing remain deferred.

Implemented private-bootstrap query routes:

```text
GET  /api/console/overview
GET  /api/models
POST /api/models/test
GET  /api/packs
GET  /api/graphs/latest
GET  /api/runtimes
GET  /api/runs
GET  /api/approvals
```

`/api/console/overview` is a read-optimized aggregate for the embedded Console. It returns only Gateway state from SQLite-backed services and should not read project files or expose raw secret values.

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

## Bootstrap Mode

Console is served by Gateway. Therefore Console-based setup needs some Gateway process to exist before normal runtimes are running.

Bootstrap mode:

```bash
nomici gateway run --setup
```

Expected behavior:

- serves Console assets
- exposes health, setup, provider catalog, model test, pack inspect, and pack install APIs
- does not start runtimes automatically
- does not run agents
- uses the same token, auth, and secret redaction rules as normal Gateway

If bootstrap mode is not implemented in the first product slice, setup remains CLI-first.

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
