# Provider Setup Design

## Purpose

Provider setup is the first useful user experience in Nomici.

It should let a user configure an LLM provider without writing raw secrets to `nomici.yaml`, test the provider, discover or record capabilities, and produce a model profile that packs and agents can use.

## Concepts

Provider Catalog:

Static or bundled definitions describing how to configure a provider.

Model Profile:

User-created configuration for one usable model endpoint.

Capability Profile:

Observed or declared capabilities for a model profile.

Secret Reference:

Reference to a secret source, never the raw secret value.

## Provider Classes

```text
native_cloud
  OpenAI, Anthropic, Gemini

openai_compatible
  OpenRouter, Together, Fireworks, Groq, custom endpoint

local_provider
  Ollama, vLLM, LM Studio, SGLang, llama.cpp server

local_auth_provider
  Codex CLI

provider_gateway
  LiteLLM or other internal gateway
```

v0.1 should implement:

- generic OpenAI-compatible
- Ollama
- Codex CLI local auth

v0.1 should define catalog entries for:

- OpenAI
- Anthropic
- Gemini
- OpenRouter
- vLLM
- LM Studio
- SGLang
- llama.cpp server

They may be marked `planned` until implemented.

## Provider Catalog Schema

```yaml
id: openai_compatible
name: OpenAI-compatible Endpoint
class: openai_compatible
status: supported
auth:
  modes:
    - api_key_env
    - none
config:
  required:
    - base_url
    - model
  optional:
    - api_key_env
    - organization
    - timeout_ms
capability_probe:
  strategy: openai_chat_test
  supports_stream_probe: true
model_discovery:
  strategy: openai_models_endpoint
defaults:
  base_url: http://127.0.0.1:8000/v1
security:
  secret_fields:
    - api_key_env
docs_url: https://nomici.ai/docs/providers/openai-compatible
```

Status values:

- `supported`
- `planned`
- `experimental`
- `deprecated`

Auth modes:

- `api_key_env`
- `keychain_ref`
- `none`
- `custom_header_env`

## Model Profile Schema

```yaml
id: local_qwen
provider_id: ollama
display_name: Local Qwen
class: local_provider
base_url: http://127.0.0.1:11434
model: qwen3:32b
auth:
  mode: none
capabilities:
  streaming: true
  tool_calling: unknown
  structured_output: unknown
  vision: false
  reasoning: unknown
  embeddings: false
limits:
  context_window: null
cost:
  input_per_1m: null
  output_per_1m: null
source:
  kind: user_configured
  checked_at: "2026-05-01T00:00:00Z"
```

Storage:

- global profiles: `~/.nomici/profiles/<profile>/models`
- project profiles: `.nomici/state.db` plus exportable references
- `nomici.yaml` may reference profile IDs but should not store raw secrets

## Setup State Machine

```text
idle
  -> provider_selected
  -> config_collected
  -> secret_reference_validated
  -> endpoint_checked
  -> capability_probed
  -> test_prompt_passed
  -> profile_saved
```

Failure states:

- `missing_secret`
- `endpoint_unreachable`
- `auth_failed`
- `model_not_found`
- `probe_failed`
- `user_cancelled`

Recovery:

- missing secret reference -> show env var export command
- endpoint unreachable: show health check and base URL remediation
- auth failed: re-enter secret reference
- model not found: list available models if supported

## CLI Behavior

```bash
nomici setup
nomici model setup
nomici model setup --provider ollama
nomici model setup --provider openai-compatible --base-url http://127.0.0.1:8000/v1
nomici model test local_qwen
nomici model doctor
nomici model list
nomici model show local_qwen --json
```

Rules:

- `nomici setup` is the first-run umbrella flow: provider profile, starter pack, sandbox policy, graph snapshot, and next-step commands.
- interactive by default
- non-interactive flags for scripts
- `--json` for machine output
- never echo raw secrets
- write only secret references

CLI setup can run before Gateway is started.

Console setup requires a Gateway process because Console is served by Gateway. v0.1 may support `nomici gateway run --setup` as a bootstrap mode; otherwise the hard setup path is CLI-first.

## First-Run Sandbox Policy

The setup flow writes explicit sandbox intent into AgentSpec:

```yaml
deployment:
  sandbox:
    mode: local
    workspace: ./workspace
    bash_enabled: false
    file_write_enabled: true
    note: v0.1 policy intent; runtime adapters enforce capabilities where supported
```

Supported setup modes:

- `local`: local workspace policy intent with approvals for risky actions
- `container`: preferred isolation intent when Docker, Podman, or Apple Container is available
- `none`: no sandbox policy intent

This keeps Nomici aligned with the long-horizon run-workspace shape without pretending Gateway is already a full sandbox runtime. `nomici doctor` checks the configured sandbox mode and warns when `container` is selected but no local container runtime is found.

## Gateway API

```text
GET  /api/providers/catalog
POST /api/providers/setup-sessions
GET  /api/providers/setup-sessions/{id}
POST /api/providers/setup-sessions/{id}/advance
POST /api/models/test
GET  /api/models
GET  /api/models/{id}
DELETE /api/models/{id}
```

Setup session response:

```json
{
  "id": "setup_01H",
  "state": "capability_probed",
  "provider_id": "ollama",
  "profile_preview": {},
  "warnings": [],
  "next_actions": ["save_profile"]
}
```

## Capability Probe

Probe outputs use tri-state values:

- `true`
- `false`
- `unknown`

Probe should test:

- endpoint reachability
- auth
- model existence if possible
- streaming if supported
- chat completion
- tool calling only where safe
- structured output only where supported

Probe must be fast and cancellable.

## Trace Events

Provider setup emits:

- `provider.setup.started`
- `provider.setup.completed`
- `provider.setup.failed`
- `model.test.started`
- `model.test.completed`
- `model.test.failed`
- `model.capabilities.probed`

These events are useful for doctor/debug but should avoid prompt content unless user opts into verbose traces.

## Security

- Raw keys are never stored in `nomici.yaml`.
- Console never receives raw secret values.
- CLI redacts secret-looking values.
- Provider test prompts are user-visible.
- Cloud provider calls are explicit.
- Local provider probes must not scan broad network ranges.

## Tests

- catalog schema validation
- profile validation
- setup state transitions
- redaction tests
- fake OpenAI-compatible endpoint test
- fake Ollama endpoint test
- CLI golden output for failures
