# Proof Slice Code Plan

This is the code-level implementation guide for Gate 1 and Gate 2 in
`docs/implementation/v0.1-plan.md`.

It expands the provider setup and minimal invoke proof slice into concrete
packages, interfaces, tests, and wiring steps. Step 0 is the scaffold
prerequisite from Gate 0 and should be completed before the Gate 1/2 work.

## Goal

Prove the narrowest vertical slice that validates Gateway, provider setup,
adapter invocation, trace store, and security defaults:

```text
provider setup
  -> OpenAI-compatible or Ollama model profile
  -> Gateway-mediated invocation
  -> trace event storage
  -> secret redaction
  -> health/status visibility
```

This slice must run without packs, AgentGraph compiler, runtime reconciler,
approval queue, A2A sidecars, or MCP proxy.

## Starting State

This was the scaffold state before implementing Gate 1 and Gate 2:

- `cmd/nomici/main.go` — binary entry point
- `internal/cli/root.go` — cobra root, only gateway subcommand
- `internal/cli/gateway.go` — `nomici gateway run`
- `internal/gateway/server.go` — HTTP server with `Options{Host, Port, Version}`
- `internal/gateway/router.go` — chi mux, `/api/health` and `/*` (Console)
- `internal/gateway/health.go` — health handler
- `internal/gateway/server_test.go` — health endpoint test
- `internal/gateway/web/web.go` — embedded Console dist
- `go.mod` — module declared; Gate 0 resolved chi and cobra dependencies before this slice started

## Step 0: Fix Dependencies (5 min)

Before any new code, get the existing scaffold to a clean build.

- Run `go mod tidy` to populate `go.mod` with chi and cobra requires
- Verify `make build` produces `bin/nomici`
- Verify `bin/nomici --version` prints version
- Verify `bin/nomici gateway run` starts and `/api/health` returns `{"status":"ok"}`
- Commit: `chore: resolve go module dependencies`

Stop here if anything fails. Do not add features until the scaffold builds cleanly.

## Step 1: SQLite Store Initialization

### Purpose
A single `*sql.DB` handle that Gateway (and later the CLI in bootstrap mode)
can pass to service packages.

### Files

`internal/store/store.go`
```go
package store

import "database/sql"

// Open opens or creates the SQLite database at the given path.
func Open(path string) (*sql.DB, error)

// Migrate applies pending schema migrations.
func Migrate(db *sql.DB) error
```

`internal/store/migrations.go`
```go
package store

// migrations is an ordered list of schema migrations.
// Migration 001 creates the provider_profiles and trace_events tables.
var migrations = []struct{ Version int; SQL string }{
    {
        Version: 1,
        SQL: `
CREATE TABLE IF NOT EXISTS provider_profiles (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL,
    base_url    TEXT NOT NULL DEFAULT '',
    model       TEXT NOT NULL DEFAULT '',
    api_key_env TEXT NOT NULL DEFAULT '',
    capabilities_json TEXT NOT NULL DEFAULT '{}',
    context_window INTEGER NOT NULL DEFAULT 0,
    cost_json   TEXT NOT NULL DEFAULT '{}',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS trace_events (
    event_id    TEXT PRIMARY KEY,
    run_id      TEXT NOT NULL,
    sequence    INTEGER NOT NULL,
    type        TEXT NOT NULL,
    time        TEXT NOT NULL DEFAULT (datetime('now')),
    node_id     TEXT NOT NULL DEFAULT '',
    runtime_id  TEXT NOT NULL DEFAULT '',
    payload_json TEXT NOT NULL DEFAULT '{}',
    redactions_json TEXT NOT NULL DEFAULT '[]',
    metadata_json   TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_trace_run ON trace_events(run_id, sequence);
`,
    },
}

// schema_version tracks which migrations have been applied.
```

`internal/store/store_test.go`
- Test: Open creates db file
- Test: Migrate applies schema idempotently
- Test: Second Migrate is a no-op

### Gateway Wiring

Add `DBPath` to `gateway.Options`:
```go
type Options struct {
    Host    string
    Port    int
    Version string
    DBPath  string  // new
}
```

Default `DBPath`: `".nomici/state.db"` relative to working directory.

Gateway server opens and migrates the DB on start. The DB handle is passed
to adapter and trace services when they are initialized.

### CLI Flag

`nomici gateway run --db-path .nomici/state.db`

### Dependencies

Add to `go.mod`: a Go SQLite driver. Recommendation: `modernc.org/sqlite`
(pure Go, no CGO, cross-platform). Also `github.com/mattn/go-sqlite3` as
an alternative if CGO is acceptable.

## Step 2: Secrets Resolver

### Purpose
Read `api_key_env` references from provider profiles, resolve the environment
variable value, and ensure the raw value is never logged, stored, or serialized.

### Files

`internal/secrets/resolver.go`
```go
package secrets

// Resolver resolves secret references to their values.
type Resolver struct{}

// NewResolver creates a resolver backed by the process environment.
func NewResolver() *Resolver

// ResolveEnv reads the named environment variable.
// Returns the value and a boolean indicating whether it was set.
func (r *Resolver) ResolveEnv(name string) (string, bool)

// Redact replaces the secret value with a placeholder suitable for
// logs, traces, and API responses.
func (r *Resolver) Redact(value string) string
```

Redaction rules:
- If the value looks like an API key (`sk-...`, `ant-...`, key longer than 20 chars), replace with `[redacted:ENV_NAME]`
- Otherwise replace with `[redacted]`
- The placeholder references the env var name so users know which credential was used, but never the value

`internal/secrets/resolver_test.go`
- Test: ResolveEnv returns value when set
- Test: ResolveEnv returns false when not set
- Test: Redact replaces sk-prefixed values
- Test: Redact replaces long values
- Test: Redact preserves short non-key-like values (config values, not secrets)

### Design Note

This is intentionally minimal. Future secret backends (keychain, 1Password CLI,
Bitwarden CLI) can implement the same interface. The v0.1 contract is: env vars
only, referenced by name, never embedded in nomici.yaml.

## Step 3: Provider Profile Storage

### Purpose
Store and retrieve LLM provider configurations without raw secrets.
The profile says *where* to find the key, not the key itself.

### Files

`internal/providers/types.go`
```go
package providers

// Profile is a configured LLM provider profile.
type Profile struct {
    ID             string            `json:"id"`
    Name           string            `json:"name"`
    Kind           string            `json:"kind"`            // openai_compatible, ollama, openai, anthropic, gemini
    BaseURL        string            `json:"base_url"`
    Model          string            `json:"model"`
    APIKeyEnv      string            `json:"api_key_env"`     // env var name, never the value
    Capabilities   map[string]string `json:"capabilities"`    // "streaming": "true", "tool_calling": "unknown"
    ContextWindow  int               `json:"context_window"`
    CostPer1MInput  float64          `json:"cost_per_1m_input,omitempty"`
    CostPer1MOutput float64          `json:"cost_per_1m_output,omitempty"`
    CreatedAt      string            `json:"created_at"`
    UpdatedAt      string            `json:"updated_at"`
}

// Validate checks that required fields are present and kind is known.
func (p *Profile) Validate() error
```

Known kinds for v0.1: `openai_compatible`, `ollama`.

`internal/providers/store.go`
```go
package providers

// Store persists and retrieves provider profiles.
type Store struct {
    db *sql.DB
}

func NewStore(db *sql.DB) *Store

// Save upserts a profile.
func (s *Store) Save(ctx context.Context, p *Profile) error

// Get retrieves a profile by ID.
func (s *Store) Get(ctx context.Context, id string) (*Profile, error)

// List returns all profiles, ordered by name.
func (s *Store) List(ctx context.Context) ([]*Profile, error)

// Delete removes a profile by ID.
func (s *Store) Delete(ctx context.Context, id string) error
```

`internal/providers/store_test.go`
- Test: Save and Get round-trip
- Test: Save updates existing
- Test: List returns all
- Test: Delete removes
- Test: Validate rejects unknown kind
- Test: Validate rejects empty ID
- Test: APIKeyEnv is stored as-is (reference), not resolved to value

## Step 4: OpenAI-Compatible Adapter

### Purpose
Call an OpenAI-compatible chat completions endpoint, return a normalized
response, and never leak the API key in the result.

### Files

`internal/adapters/adapter.go`
```go
package adapters

// InvokeRequest is the input to an adapter invocation.
type InvokeRequest struct {
    RunID     string
    NodeID    string
    Messages  []Message
    Options   InvokeOptions
    TraceContext *TraceContext
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type InvokeOptions struct {
    Stream    bool
    TimeoutMs int
}

type TraceContext struct {
    ParentEventID string
}

// InvokeResult is the normalized output from an adapter invocation.
type InvokeResult struct {
    Status    string       `json:"status"`    // completed, failed
    Messages  []Message    `json:"messages,omitempty"`
    Usage     *UsageInfo   `json:"usage,omitempty"`
    Error     *AdapterError `json:"error,omitempty"`
}

type UsageInfo struct {
    InputTokens  int     `json:"input_tokens"`
    OutputTokens int     `json:"output_tokens"`
    CostUSD      *float64 `json:"cost_usd,omitempty"`
}

type AdapterError struct {
    Code        string `json:"code"`
    Message     string `json:"message"`
    Retryable   bool   `json:"retryable"`
}
```

`internal/adapters/openai_compatible.go`
```go
package adapters

// OpenAICompatibleAdapter calls OpenAI-compatible chat completion endpoints.
type OpenAICompatibleAdapter struct {
    httpClient *http.Client
}

func NewOpenAICompatibleAdapter() *OpenAICompatibleAdapter

// Invoke sends a chat completion request and returns a normalized result.
// The apiKey is passed separately (resolved at call time by Gateway) and
// is never stored in the adapter or returned in the result.
func (a *OpenAICompatibleAdapter) Invoke(
    ctx context.Context,
    baseURL string,
    model string,
    apiKey string,
    req InvokeRequest,
) (*InvokeResult, error)
```

Implementation details:
- POST to `{baseURL}/chat/completions` (strip trailing `/v1` first if present, or require caller to pass clean URL)
- Set `Authorization: Bearer {apiKey}`
- Parse the OpenAI chat completion response
- Map `choices[0].message` to `InvokeResult.Messages`
- Extract `usage.prompt_tokens`, `usage.completion_tokens`
- On HTTP error or JSON parse failure, return `InvokeResult{Status: "failed", Error: ...}`
- On non-2xx response, read body, return AdapterError with code `endpoint_unavailable` or `auth_failed` based on status (401/403 → auth_failed, others → endpoint_unavailable)
- Timeout from `InvokeOptions.TimeoutMs` or default 120s

`internal/adapters/openai_compatible_test.go`
- Test: successful invocation with httptest server
- Test: 401 returns auth_failed error
- Test: 500 returns endpoint_unavailable error
- Test: timeout returns error
- Test: API key is not present in result or error message
- Test: model name is sent in the request body

### Design Note

This adapter uses the OpenAI chat completions format directly. It does not
implement the full `Adapter` interface from RFC 0007 yet — no stream, cancel,
health, capabilities, or logs methods. Those can be added after the proof slice
when the interface is proven against a real endpoint.

## Step 5: Trace Event Store

### Purpose
Record structured, append-only trace events for every model invocation.

### Files

`internal/trace/types.go`
```go
package trace

import "time"

// Event is one trace event in a run.
type Event struct {
    EventID    string          `json:"event_id"`
    RunID      string          `json:"run_id"`
    Sequence   int             `json:"sequence"`
    Type       string          `json:"type"`
    Time       time.Time       `json:"time"`
    NodeID     string          `json:"node_id,omitempty"`
    RuntimeID  string          `json:"runtime_id,omitempty"`
    Payload    json.RawMessage `json:"payload"`
    Redactions []string        `json:"redactions"`
    Metadata   json.RawMessage `json:"metadata,omitempty"`
}
```

Proof slice event types:
- `run.started`
- `model.requested`
- `model.completed`
- `model.failed`
- `run.completed`
- `run.failed`

`internal/trace/store.go`
```go
package trace

// Store persists trace events.
type Store struct {
    db *sql.DB
}

func NewStore(db *sql.DB) *Store

// Append writes one trace event with the next sequence number for its run.
func (s *Store) Append(ctx context.Context, event *Event) error

// ListByRun returns all events for a run, ordered by sequence.
func (s *Store) ListByRun(ctx context.Context, runID string) ([]*Event, error)

// NextSequence returns the next sequence number for a run (max+1, or 1).
func (s *Store) nextSequence(ctx context.Context, runID string) (int, error)
```

`internal/trace/store_test.go`
- Test: Append writes event with auto-incremented sequence
- Test: ListByRun returns events in order
- Test: Separate runs have independent sequences
- Test: EventID is unique constraint violation on duplicate
- Test: Payload JSON round-trips

## Step 6: Wire It Together — Gateway Model Test Endpoint

### Purpose
A single Gateway API endpoint that accepts a provider profile ID and a prompt,
resolves the secret, invokes the adapter, stores trace events, and returns
a redacted result.

### New Gateway API Endpoint

```
POST /api/models/test
```

Request:
```json
{
  "provider_id": "gpt",
  "prompt": "Say hello in exactly three words.",
  "stream": false
}
```

Response (success, 200):
```json
{
  "data": {
    "run_id": "run_01H...",
    "status": "completed",
    "messages": [
      {"role": "assistant", "content": "Hello, nice day."}
    ],
    "usage": {
      "input_tokens": 15,
      "output_tokens": 5
    },
    "trace_event_count": 3
  },
  "warnings": [],
  "request_id": "req_01H..."
}
```

Response (error, 4xx/5xx):
```json
{
  "error": {
    "code": "auth_failed",
    "message": "Provider returned 401. Check your API key.",
    "remediation": "Verify the OPENAI_API_KEY environment variable is set and valid.",
    "details": {}
  },
  "request_id": "req_01H..."
}
```

### Gateway Internal Flow for POST /api/models/test

```
1. Parse request body
2. Validate provider_id is not empty, prompt is not empty
3. Load provider profile from providers.Store
4. Resolve api_key_env via secrets.Resolver
5. If env var not set, return error {code: "missing_secret", message: "Env var X not set"}
6. Generate run_id (ULID or UUID)
7. Write trace event: run.started
8. Write trace event: model.requested (payload includes prompt, provider_id, model — API key redacted)
9. Call OpenAICompatibleAdapter.Invoke(ctx, profile.BaseURL, profile.Model, apiKey, request)
10. If adapter fails:
    a. Write trace event: model.failed (payload includes redacted error)
    b. Write trace event: run.failed
    c. Return error response
11. If adapter succeeds:
    a. Write trace event: model.completed (payload includes usage, response summary — full messages redacted by default in trace, stored by reference)
    b. Write trace event: run.completed
    c. Return success response with messages and usage
```

### Files Modified

`internal/gateway/server.go`
- Add `*sql.DB` field to Server
- Open DB in constructor or Run method
- Pass DB to handler dependencies

`internal/gateway/router.go`
- Add `POST /api/models/test` route
- Pass `*sql.DB` or service dependencies to handler

`internal/gateway/model_test_handler.go` (new)
```go
package gateway

func modelTestHandler(db *sql.DB, version string) http.HandlerFunc {
    // Implements the flow described above
}
```

### Redaction in Trace Payloads

The `model.requested` trace event payload includes:
```json
{
  "provider_id": "gpt",
  "model": "gpt-5.5",
  "base_url": "https://api.openai.com/v1",
  "prompt": "Say hello in exactly three words.",
  "api_key_source": "OPENAI_API_KEY"
}
```

Note: `api_key_source` records the env var name, not the value. The resolved
API key is never serialized into any trace payload or response body.

### Request ID

Generate a short unique request ID (e.g., `req_` + 8 hex chars from crypto/rand)
for each API call. Include in response envelope and trace metadata for correlation.

## Step 7: CLI — Model Setup

### Purpose
Interactive or flag-driven CLI to create a provider profile.

### Files

`internal/cli/model.go`
```go
package cli

func newModelCommand() *cobra.Command {
    // nomici model
    // Subcommands: setup, test, list
}

func newModelSetupCommand(dbPath string) *cobra.Command {
    // nomici model setup
    // Interactive prompts:
    //   ? Provider kind: openai_compatible / ollama
    //   ? Name for this profile: gpt
    //   ? Base URL: [https://api.openai.com/v1]
    //   ? Model: gpt-5.5
    //   ? API key env var: OPENAI_API_KEY
    // Saves profile via providers.Store
}
```

For v0.1, keep it simple: accept flags for non-interactive use, fall back to
interactive prompts if stdin is a terminal.

```bash
# Interactive
nomici model setup

# Non-interactive
nomici model setup \
  --kind openai_compatible \
  --name gpt \
  --base-url https://api.openai.com/v1 \
  --model gpt-5.5 \
  --api-key-env OPENAI_API_KEY
```

`internal/cli/model_setup.go`
- Read flags or prompt interactively
- Construct providers.Profile
- Open store.DB (bootstrap mode — CLI opens DB directly, no Gateway needed)
- Call store.Save
- Print confirmation

### Bootstrap vs Client Mode

`nomici model setup` runs in bootstrap mode: it opens the SQLite DB directly.
It does not require a running Gateway. This is consistent with the settled
decision that setup commands may run before Gateway is available.

`nomici model test` runs in client mode: it posts to the Gateway API. It
requires a running Gateway. If Gateway is not reachable, print a clear error
with remediation (`nomici gateway run`).

## Step 8: CLI — Model Test

### Purpose
Call the Gateway model test endpoint from the CLI and display the result.

### Files

`internal/cli/model_test.go`
```go
func newModelTestCommand(gatewayURL string) *cobra.Command {
    // nomici model test <profile_id> [prompt]
    // POST to {gatewayURL}/api/models/test
    // Format and print response
}
```

```bash
nomici model test gpt "Say hello in three words."
```

Output on success:
```text
Run ID:    run_01HABC...
Status:    completed
Response:  Hello, nice day.
Tokens:    15 in / 5 out
Trace:     3 events stored
```

Output on failure:
```text
Error: auth_failed
Provider returned 401. Check your API key.
Remediation: Verify the OPENAI_API_KEY environment variable is set and valid.
```

### Gateway URL

Default: `http://127.0.0.1:8787`. Overridable via `--gateway-url` flag or
`NOMICI_GATEWAY_URL` env var.

## Step 9: CLI — Model List

### Purpose
List configured provider profiles.

### Files

`internal/cli/model_list.go`
```bash
nomici model list
```

Output:
```text
ID    NAME   KIND                MODEL              API KEY SOURCE
gpt   gpt    openai_compatible   gpt-5.5            OPENAI_API_KEY
local local  ollama              qwen3:32b          (none)
```

Note: API key values are never displayed. Only the env var name is shown.

## Step 10: Put It All Together — Integration Test

### Purpose
One end-to-end test that exercises the full proof slice.

### Files

`internal/gateway/model_test_integration_test.go`

```go
func TestModelTestIntegration(t *testing.T) {
    // 1. Start a fake OpenAI-compatible server (httptest)
    // 2. Create a temp SQLite DB
    // 3. Insert a test provider profile
    // 4. Start the Gateway with the temp DB
    // 5. POST /api/models/test
    // 6. Assert 200, messages present, usage present
    // 7. Query trace_events table, assert 3+ events
    // 8. Assert no raw secret in trace payloads or response
}
```

This test alone validates: store, providers, secrets, adapters, trace, and
the Gateway HTTP contract — without manual setup.

### CLI Smoke Test

A shell-level smoke test (in `scripts/smoke-test.sh` or similar):

```bash
# Start fake server, set env, run CLI commands, check exit codes
```

This is optional for the proof slice and can be added after the integration
test passes.

## Implementation Order

Each step should produce a passing test before moving to the next.

```
Step 0  — Fix deps, verify scaffold builds
Step 1  — SQLite init + migration (store package)
Step 2  — Secrets resolver
Step 3  — Provider profile store
Step 4  — OpenAI-compatible adapter (with httptest)
Step 5  — Trace event store
Step 6  — Gateway model test endpoint (wiring + handler)
Step 7  — CLI model setup (bootstrap mode)
Step 8  — CLI model test (client mode)
Step 9  — CLI model list
Step 10 — Integration test (end-to-end)
```

Steps 2-5 have no dependency on each other and can be implemented in any order
or in parallel. Steps 6-9 depend on 1-5. Step 10 depends on 6.

## What This Slice Does NOT Include

- Pack manifests or pack installation
- AgentSpec YAML parsing or AgentGraph compilation
- Runtime registry, runtime reconciler, or process management
- Approval queue or policy decisions (model test has no side-effecting tools)
- Shared context items or handoff snapshots
- Console UI (Console embed is already in scaffold, but shows nothing useful yet)
- CLI agent runner (Claude Code, Codex, etc.)
- Streaming responses (the adapter sends `stream: false`)
- Gateway token auth (loopback only, auth deferred to later hardening)

## What This Slice PROVES

- Gateway can mediate an LLM call without raw secrets in flight
- Adapter abstraction works for the simplest case (HTTP chat completion)
- Trace events are append-only and queryable
- CLI has clear bootstrap/client mode separation
- Response envelope is used for all non-health endpoints
- Secret redaction is verifiable by test, not by policy

## Success Criteria

```bash
# Setup
export OPENAI_API_KEY=<your_api_key>
nomici model setup --kind openai_compatible --name gpt --model gpt-5.5 --api-key-env OPENAI_API_KEY

# Start Gateway
nomici gateway run &
sleep 1
curl http://127.0.0.1:8787/api/health

# Test model through Gateway
nomici model test gpt "Say hello in three words."
# Output: Run ID, status completed, response message, token counts

# Test model through curl
curl -s -X POST http://127.0.0.1:8787/api/models/test \
  -H "Content-Type: application/json" \
  -d '{"provider_id":"gpt","prompt":"Say hello"}'

# Verify traces exist (manual query or future nomici trace list)
# Verify no API key in traces, logs, or responses
```

## File Manifest

New files:
```
internal/store/store.go
internal/store/migrations.go
internal/store/store_test.go
internal/secrets/resolver.go
internal/secrets/resolver_test.go
internal/providers/types.go
internal/providers/store.go
internal/providers/store_test.go
internal/adapters/adapter.go
internal/adapters/openai_compatible.go
internal/adapters/openai_compatible_test.go
internal/trace/types.go
internal/trace/store.go
internal/trace/store_test.go
internal/gateway/model_test_handler.go
internal/gateway/model_test_integration_test.go
internal/cli/model.go
internal/cli/model_setup.go
internal/cli/model_test.go
internal/cli/model_list.go
```

Modified files:
```
go.mod                              — add require directives
internal/gateway/server.go          — add DBPath, *sql.DB field
internal/gateway/router.go          — add POST /api/models/test
internal/cli/root.go               — add model subcommand
Makefile                            — optional: add test-db target
```

## Go Dependencies to Add

```
modernc.org/sqlite        — pure-Go SQLite driver
github.com/google/uuid     — or use crypto/rand for ID generation
```

Alternatives:
- `github.com/mattn/go-sqlite3` if CGO is acceptable
- ULID library (`github.com/oklog/ulid/v2`) for sortable IDs, or just UUID v4
