# Basic Local Agent

This example runs a local command-backed `cli_agent` without an API key.

It proves the smallest useful Nomici path:

```text
AgentSpec -> AgentGraph -> cli_agent runtime -> trace/artifacts
```

Run it from this directory:

```bash
nomici spec validate --config nomici.yaml
nomici graph validate --config nomici.yaml
nomici run local_assistant "Explain what this example demonstrates." --config nomici.yaml
nomici trace list
```

The runtime uses `sh`, so it is intended for macOS/Linux/WSL.
