# Troubleshooting

## `nomici dev` says no project config exists

Run setup from the project root:

```bash
nomici setup
nomici dev
```

`nomici.yaml` is shared project config. Local provider profiles, Gateway token, runtime state, and secrets stay under `.nomici/` or ignored local overrides.

## Console asks for a token

Run this from the same project directory where Gateway started:

```bash
nomici gateway token show
```

Paste that value into Console. Do not commit the token.

## Provider model list fails

Check the selected provider and auth reference:

```bash
nomici provider list
nomici provider doctor <provider_id>
nomici provider models <provider_id> --refresh
```

For OpenAI-compatible endpoints, verify `base_url`, the env var name, and whether `/v1/models` is supported. If the provider cannot enumerate models, use an explicit model id during setup.

## A run is blocked

Open the Chat workspace panel and look at `Needs input`. Common blockers:

- `plan_review`: approve or revise the plan.
- `tool_approval`: grant or deny the approval.
- `retry_decision`: retry, skip, or stop after a repeated tool failure or budget stop.
- `clarification`: answer the missing input question.

After resolving the action, Console resumes the session when it is safe. CLI users can inspect state with:

```bash
nomici session show <session_id>
nomici approvals list
```

## File or bash tools fail

Check the session sandbox policy and workspace roots in Console. Mutating tools require:

- a session sandbox record
- workspace path inside the session workspace
- file-write or bash permission enabled
- approval unless a scoped grant already exists
- no critical command pattern for bash

Container bash execution requires Docker or Podman and an explicit container sandbox.

## Artifacts have preview but no downloadable file

Some artifacts are text-only records. Download is available when the artifact has a file path inside the session workspace or artifact root. Use the preview for text-only plan/report artifacts.

## Memory is not reused

Only approved memory proposals become reusable context. Review memory proposals in Console, approve the useful ones, and delete stale approved items when they no longer apply.
