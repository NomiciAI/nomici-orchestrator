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
- preparing the built-in default team
- writing a commit-safe `nomici.yaml` project manifest with local provider profile references
- writing sandbox policy intent to `deployment.sandbox`
- saving or materializing graph metadata for Gateway and Console

`nomici.yaml` is the shared project manifest. It should contain shared agent/team choices, tool contracts, and policy intent, but not local provider URLs, raw keys, or personal runtime state. `nomici setup` stores those local provider details in `.nomici/`; optional local AgentSpec overrides belong in ignored `nomici.local.yaml`. If no explicit team graph exists yet, Gateway can create the default coordinator/planner/researcher/coder/reporter team from the first configured model profile when Chat starts a workspace run.

The same flow is scriptable for automation:

```bash
nomici setup \
  --provider openai \
  --name gpt \
  --model <model> \
  --api-key-env OPENAI_API_KEY \
  --web-search duckduckgo \
  --web-fetch jina-reader \
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

For local Codex CLI auth, the setup wizard shows the provider only when it is ready to run. Nomici checks `codex` on `PATH`, then checks the macOS app-bundle executable, and uses `CODEX_HOME/auth.json` when `CODEX_HOME` is set, otherwise the current OS user home path such as `/Users/<you>/.codex/auth.json`, `/home/<you>/.codex/auth.json`, or `C:\Users\<you>\.codex\auth.json`. Use `nomici provider list --all` or `nomici provider doctor codex-cli` to diagnose missing local auth or executable paths.

You can also script it explicitly after local auth is ready:

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

Sandbox modes are `local`, `container`, and `none`. Runtime adapters enforce sandbox capabilities where supported. Tool Broker can execute file, bash, search, and fetch requests through policy, approval, redaction, trace, and artifact records. Model adapters use native tool schemas where supported and fall back to JSON tool requests otherwise. `nomici doctor` checks that the project manifest stays commit-safe, that sandbox config exists, warns if `container` is selected but Docker, Podman, or Apple `container` is not available, and reports missing env vars for configured providers.

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

`nomici dev` starts Gateway, validates the graph, starts configured local processes, and opens Console. Console defaults to Chat; a message can start a workspace run, and the workspace panel shows route decisions, role flow, blocked actions, tool calls, plan review, uploads, artifacts and revisions, trace events, approvals, and memory proposals for the active session. Orchestrate shows the review queue and role flow builder. Settings shows provider/model catalog status, configured model profiles, agent builder controls, and tool/skill selection. `nomici up` remains available for scripts that only need the lower-level background start behavior.

Useful inspection commands after a run:

```bash
nomici review list
nomici artifact revisions <artifact_id>
nomici memory items
nomici eval router
```

When finished:

```bash
nomici down
```

## Optional: Configure An LLM Provider

Scriptable provider setup stores only the environment variable name, not the raw key. The interactive `nomici setup` wizard also accepts a pasted provider key and saves it to ignored `.nomici/secrets.env` while keeping `nomici.yaml` commit-safe.

```bash
export OPENAI_API_KEY=...
nomici model setup --kind openai_compatible --name gpt --model <model> --api-key-env OPENAI_API_KEY
nomici model test gpt "Say hello from Nomici."
```

This lower-level command remains useful for scripts and advanced setups. For a new workspace, prefer `nomici setup` because it also prepares the default team and configures sandbox policy.

You can also test the OpenAI-compatible Gateway surface:

```bash
nomici up
curl -H "Authorization: Bearer $(nomici gateway token show)" \
  http://127.0.0.1:8787/v1/models
nomici down
```

## Optional: Inspect The Default Team

The built-in default team is available without a manual install step. Advanced users can inspect the team before copying or customizing it:

```bash
nomici agent list
nomici orchestrate show
nomici run product_pm "Draft a tiny implementation plan."
```

Console's Agent and Orchestration views are the preferred way to turn the default team into a project-specific team.

## Current Limits

- The hosted `curl` installer and release artifacts are not live yet.
- Console can edit agents and sequential role flow; provider bootstrap still starts with `nomici setup` and `nomici provider` commands.
- Linear `handoff` chains across `cli_agent`-backed `external_agent` nodes are executable. Branching, parallel, A2A, and static tool-edge graph execution are not implemented yet.
- Model-backed roles can call mediated file, bash, search, and fetch tools. Broader MCP/browser/office/channel tool adapters remain later hardening work.
- Server/team mode, signed remote pack distribution, and deep external runtime adapters are later hardening work.
