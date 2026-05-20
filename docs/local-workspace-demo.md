# Local Workspace Demo Script

This script is the shortest end-to-end demo for a fresh local checkout.

## Setup

```bash
scripts/install.sh --from-source .
nomici setup
nomici doctor
nomici dev
```

Open Console from the URL printed by `nomici dev`, paste the Gateway token when prompted, and start in Chat.

## Demo Flow

1. Send: `Inspect this project, create a short implementation plan, and produce a final report artifact.`
2. Confirm that the workspace panel shows a route decision, selected roles, task ledger, workspace roots, and event stream.
3. When a plan review appears, revise it if needed and approve it.
4. Upload a small text file from the Uploads panel.
5. Send a follow-up asking Nomici to read the upload, update a workspace file, run verification, and summarize the result.
6. Grant the requested tool approval.
7. Confirm that tool calls show file/bash execution, output previews, trace events, and artifact references.
8. Open the Artifacts panel, preview the final report, inspect revisions, and download any file-backed artifact.
9. Open Orchestrate and confirm the review queue points back to the blocked or waiting session when action is needed.
10. Approve or reject the memory proposal.
11. Refresh the browser and verify the same chat, session, tasks, artifacts, revisions, tool calls, and blocked actions are still visible.

## CLI Cross-Checks

```bash
nomici session list
nomici session show <session_id>
nomici session tasks <session_id>
nomici artifact list --session <session_id>
nomici artifact revisions <artifact_id>
nomici review list
nomici eval router
nomici trace show <run_id>
nomici approvals list
```

## Expected Behavior

- Chat is the entrypoint; task/session/trace records are execution details.
- Mutating file and bash tools request approval unless policy already grants the run.
- Tool output is redacted before it is stored in trace previews.
- If a tool loop repeats a failing call, requests a critical command, or exhausts its round budget, the session enters a blocked state with a visible retry or risk review decision.
- Approved memory is available to later runs and is shown as reusable context.
