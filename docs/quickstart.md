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
nomici doctor
nomici setup
nomici up
nomici gateway open
```

If `nomici` is not on your `PATH`, add `~/.local/bin` or run `./bin/nomici` from the repository root.

## Guided Setup

The recommended first-run path is:

```bash
nomici setup
nomici doctor
nomici up
nomici run product_pm "Plan the first useful task."
```

`nomici setup` guides you through:

- choosing an LLM provider profile
- installing the `developer-team` starter pack
- writing sandbox policy intent to `deployment.sandbox`
- saving a graph snapshot for Gateway and Console

The same flow is scriptable for automation:

```bash
nomici setup \
  --provider openai-compatible \
  --name gpt \
  --model <model> \
  --api-key-env OPENAI_API_KEY \
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
  --sandbox container \
  --enable-bash \
  --enable-file-write \
  --yes
```

Sandbox modes are `local`, `container`, and `none`. In v0.1 this config is explicit control-plane policy metadata; runtime adapters enforce sandbox capabilities where supported. `nomici doctor` checks that sandbox config exists and warns if `container` is selected but Docker, Podman, or Apple `container` is not available.

## Run A Local Agent Without An API Key

The smallest demo uses a local `cli_agent` runtime. It does not call an LLM provider; it proves that Nomici can load AgentSpec, execute a local agent command, capture artifacts, and write traces.

```bash
cd examples/basic-local-agent
nomici spec validate --config nomici.yaml
nomici graph validate --config nomici.yaml
nomici run local_assistant "Summarize what this demo proves." --config nomici.yaml
nomici trace list
```

The output should include a run id and a response from the local command-backed agent.

## Start Gateway And Console

From any project with a `nomici.yaml`:

```bash
nomici up --config nomici.yaml
nomici gateway token show
nomici gateway open
```

Paste the token into Nomici Console when prompted. Run `nomici gateway token show` from the same project directory where `nomici up` started Gateway. Each `.nomici` state directory has its own Gateway token.

The current Console is read-only. It shows the latest graph snapshot saved by `nomici up`, `nomici graph validate`, `nomici run`, or `nomici pack install`.

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
- Console editing and provider setup are not implemented yet.
- Linear `handoff` chains across `cli_agent`-backed `external_agent` nodes are executable. Branching, parallel, A2A, and tool-edge graph execution is not implemented yet.
- MCP, A2A, broader tool policy, and deep Hermes/OpenClaw adapters are deferred.
