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
nomici run local_assistant "Explain what this example demonstrates." --config examples/basic-local-agent/nomici.yaml
```

If `nomici` is not on your `PATH`, add `~/.local/bin` or run `./bin/nomici` from the repository root.

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
- General multi-node graph execution is not implemented yet.
- MCP, A2A, broader tool policy, and deep Hermes/OpenClaw adapters are deferred.
