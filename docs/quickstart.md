# Quickstart

This quickstart uses the current alpha bootstrap build. It does not require a hosted installer or release artifact.

For now, installation means "build Nomici from this source checkout and copy the `nomici` binary to `~/.local/bin`." A normal one-line installer and release binaries are still planned.

## Prerequisites

- Go
- Node.js
- macOS or Linux shell for the local CLI-agent example

Why Node.js? The `nomici` CLI and Gateway are written in Go, but the bundled Console is a React app. The source installer builds that Console before compiling the Go binary. If `pnpm` is missing, the installer tries to activate it through Node.js Corepack and prints a fix command if that fails.

Clone the repo and install from source:

```bash
git clone https://github.com/NomiciAI/nomici-orchestrator.git
cd nomici-orchestrator
scripts/install.sh --from-source .
nomici setup
nomici dev
```

If `nomici` is not on your `PATH`, add `~/.local/bin` or run `./bin/nomici` from the repository root.

## Guided Setup

The recommended first-run path is:

```bash
nomici setup
nomici doctor
nomici dev
nomici run product_pm "Plan the first useful task."
```

`nomici setup` guides you through:

- choosing an LLM provider and a provider-specific model from the live catalog when available
- choosing basic Web Search and Web Fetch providers
- installing the `developer-team` starter pack
- writing a commit-safe `nomici.yaml` project manifest with local provider profile references
- writing sandbox policy intent to `deployment.sandbox`
- saving a graph snapshot for Gateway and Console

`nomici.yaml` is the shared project manifest. It should contain packs, agents, tool contracts, and policy intent, but not local provider URLs, raw keys, or personal runtime state. `nomici setup` stores those local provider details in `.nomici/`; optional local AgentSpec overrides belong in ignored `nomici.local.yaml`.

The same flow is scriptable for automation:

```bash
nomici setup \
  --provider openai \
  --name gpt \
  --model <model> \
  --api-key-env OPENAI_API_KEY \
  --web-search duckduckgo \
  --web-fetch jina-reader \
  --pack developer-team \
  --sandbox local \
  --enable-file-write \
  --yes
```

For local Ollama:

```bash
nomici setup \
  --provider ollama \
  --name local-llama \
  --model llama3.2 \
  --web-search duckduckgo \
  --web-fetch jina-reader \
  --sandbox container \
  --enable-bash \
  --enable-file-write \
  --yes
```

For local Codex CLI auth, use the setup wizard when `codex` is on `PATH` and local auth is available, or script it explicitly:

```bash
nomici setup \
  --provider codex-cli \
  --name codex-local \
  --model gpt-5.4 \
  --web-search duckduckgo \
  --web-fetch jina-reader \
  --sandbox local \
  --enable-file-write \
  --yes
```

You can inspect the full provider/model catalog outside the wizard:

```bash
nomici provider list
nomici provider models openai --search gpt
nomici provider models openrouter --search claude
nomici provider doctor openai
```

Provider catalogs are fetched from provider model APIs when available. Local CLI providers and custom endpoints still allow an explicit model id when a model list cannot be discovered.

Sandbox modes are `local`, `container`, and `none`. In v0.1 this config is explicit control-plane policy metadata; runtime adapters enforce sandbox capabilities where supported. Web Search and Web Fetch setup writes read-only provider contracts; full mediated tool execution is a later workflow. `nomici doctor` checks that the project manifest stays commit-safe, that sandbox config exists, warns if `container` is selected but Docker, Podman, or Apple `container` is not available, and reports missing env vars for configured providers.

## Optional: Run A Local Agent Without An API Key

The smallest no-provider smoke test uses a local `cli_agent` runtime. It is optional and not part of the normal first-run path. It proves that Nomici can load AgentSpec, execute a local agent command, capture artifacts, and write traces.

```bash
nomici spec validate --config examples/basic-local-agent/nomici.yaml
nomici graph validate --config examples/basic-local-agent/nomici.yaml
nomici run local_assistant "Summarize what this demo proves." --config examples/basic-local-agent/nomici.yaml
nomici trace list
```

The output should include a run id and a response from the local command-backed agent.

## Start Gateway And Console

From any project with a `nomici.yaml`:

```bash
nomici dev --config nomici.yaml
nomici gateway token show
```

Paste the token into Nomici Console when prompted. Run `nomici gateway token show` from the same project directory where `nomici dev` started Gateway. Each `.nomici` state directory has its own Gateway token.

`nomici dev` starts Gateway, validates the graph, starts configured local processes, and opens Console. Console defaults to Chat; a message can start a workspace run, and Orchestrate shows the task ledger, plan review, uploads, artifacts, trace events, and approvals for the active session. Settings shows provider/model catalog status, configured model profiles, and tool contracts. `nomici up` remains available for scripts that only need the lower-level background start behavior.

When finished:

```bash
nomici down
```

## Optional: Configure An LLM Provider

Provider setup stores only the environment variable name, not the raw key.

```bash
export OPENAI_API_KEY=...
nomici model setup --kind openai_compatible --name gpt --model <model> --api-key-env OPENAI_API_KEY
nomici model test gpt "Say hello from Nomici."
```

This lower-level command remains useful for scripts and advanced setups. For a new workspace, prefer `nomici setup` because it also installs a starter pack and configures sandbox policy.

You can also test the OpenAI-compatible Gateway surface:

```bash
nomici up
curl -H "Authorization: Bearer $(nomici gateway token show)" \
  http://127.0.0.1:8787/v1/models
nomici down
```

## Optional: Install The Developer Team Pack

After configuring at least one model profile:

```bash
nomici pack install developer-team --model gpt
nomici run product_pm "Draft a tiny implementation plan."
nomici trace show <run_id>
```

`pack install` writes the pack into `nomici.yaml` and saves a graph snapshot for Console. If Console is already open, refresh it after installing the pack.

## Current Limits

- The hosted `curl` installer and release artifacts are not live yet.
- Console setup editing is not implemented yet; use `nomici setup` and `nomici provider` commands for bootstrap changes.
- Linear `handoff` chains across `cli_agent`-backed `external_agent` nodes are executable. Branching, parallel, A2A, and tool-edge graph execution is not implemented yet.
- Web Search and Web Fetch are configured as read-only provider contracts; mediated runtime tool execution is deferred.
- MCP, A2A, broader tool policy, and deep external runtime adapters are deferred.
