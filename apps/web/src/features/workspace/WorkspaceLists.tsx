import type { ConsoleState } from "../../hooks/useChatWorkspace";
import { eventOutput, formatTime } from "../../lib/format";

export function WorkspaceLists({ state }: { state: ConsoleState }) {
  const {
    sessionDetail,
    traceEvents,
    workspaceUploads: uploads,
    workspaceArtifacts: artifacts,
    workspaceToolCalls: toolCalls,
    overview,
    memoryProposals,
    memoryItems,
    artifactContent,
    artifactRevisions,
    artifactMutation,
    uploadFile,
    setUploadFile,
    workspaceMutation,
    mutatingApproval,
    mutatingMemory,
    uploadInput,
    resolveApproval,
    resolveMemory,
    deleteMemoryItem,
    inspectArtifact,
    downloadArtifact,
  } = state;
  const approvals = overview.pending_approvals;

  return (
    <div className="workspace-lists">
      <div>
        <div className="mini-heading">
          <strong>Uploads</strong>
          <span>{uploads.length}</span>
        </div>
        <div className="upload-box">
          <input
            type="file"
            onChange={(event) => setUploadFile(event.target.files?.[0] ?? null)}
          />
          <button
            className="button button-secondary"
            type="button"
            disabled={!uploadFile || !sessionDetail || workspaceMutation !== ""}
            onClick={uploadInput}
          >
            Add
          </button>
        </div>
        {uploads.slice(0, 4).map((upload) => (
          <div className="event-row passive-row" key={upload.upload_id}>
            <span>{upload.filename}</span>
            <strong>{upload.status}</strong>
          </div>
        ))}
      </div>
      <div>
        <div className="mini-heading">
          <strong>Artifacts</strong>
          <span>{artifacts.length}</span>
        </div>
        {artifacts.slice(0, 5).map((artifact) => (
          <div className="artifact-row" key={artifact.artifact_id}>
            <div>
              <span>
                {artifact.type} / {artifact.review_state} / r{artifact.revision}
              </span>
              <strong>{artifact.title}</strong>
              {artifact.task_id ? <small>{artifact.task_id}</small> : null}
            </div>
            <div>
              <button
                className="button button-secondary"
                type="button"
                disabled={artifactMutation !== ""}
                onClick={() => inspectArtifact(artifact.artifact_id)}
              >
                Preview
              </button>
              <button
                className="button button-secondary"
                type="button"
                disabled={artifactMutation !== "" || !artifact.path}
                onClick={() => downloadArtifact(artifact)}
              >
                Download
              </button>
            </div>
          </div>
        ))}
        {artifactContent ? (
          <div className="artifact-detail">
            <div className="mini-heading no-border">
              <strong>{artifactContent.artifact_id}</strong>
              <span>{artifactContent.truncated ? "truncated" : "full"}</span>
            </div>
            <code>{artifactContent.path}</code>
            <pre>{artifactContent.content}</pre>
            {artifactRevisions.length > 0 ? (
              <div className="revision-list">
                <div className="mini-heading no-border">
                  <strong>Revisions</strong>
                  <span>{artifactRevisions.length}</span>
                </div>
                {artifactRevisions.slice(0, 5).map((revision) => (
                  <div
                    className="event-row passive-row"
                    key={revision.revision_id}
                  >
                    <span>
                      r{revision.revision} / {revision.review_state}
                    </span>
                    <strong>{formatTime(revision.created_at)}</strong>
                    {revision.diff_preview ? (
                      <small>{revision.diff_preview}</small>
                    ) : null}
                  </div>
                ))}
              </div>
            ) : null}
          </div>
        ) : null}
      </div>
      <div>
        <div className="mini-heading">
          <strong>Events</strong>
          <span>{traceEvents.length}</span>
        </div>
        {traceEvents.slice(-6).map((event) => (
          <div className="event-row passive-row" key={event.event_id}>
            <span>{event.type}</span>
            <strong>{eventOutput(event) || formatTime(event.time)}</strong>
          </div>
        ))}
      </div>
      <div>
        <div className="mini-heading">
          <strong>Tool calls</strong>
          <span>{toolCalls.length}</span>
        </div>
        {toolCalls.slice(0, 5).map((call) => (
          <div className="event-row passive-row" key={call.tool_call_id}>
            <span>{call.tool_id}</span>
            <strong>
              {call.status}
              {call.approval_id ? ` / ${call.approval_id}` : ""}
            </strong>
            {call.output_preview || call.error ? (
              <small>{call.output_preview || call.error}</small>
            ) : null}
          </div>
        ))}
      </div>
      <div>
        <div className="mini-heading">
          <strong>Approvals</strong>
          <span>{approvals.length}</span>
        </div>
        {approvals.slice(0, 3).map((approval) => (
          <div className="approval-card" key={approval.approval_id}>
            <strong>{approval.summary}</strong>
            <span>{approval.risk}</span>
            <div>
              <button
                className="button button-secondary"
                type="button"
                disabled={mutatingApproval !== ""}
                onClick={() => resolveApproval(approval.approval_id, "grant")}
              >
                Grant
              </button>
              <button
                className="button button-danger"
                type="button"
                disabled={mutatingApproval !== ""}
                onClick={() => resolveApproval(approval.approval_id, "deny")}
              >
                Deny
              </button>
            </div>
          </div>
        ))}
      </div>
      <div>
        <div className="mini-heading">
          <strong>Memory</strong>
          <span>{memoryProposals.length + memoryItems.length}</span>
        </div>
        {memoryProposals.slice(0, 3).map((proposal) => (
          <div className="approval-card" key={proposal.proposal_id}>
            <strong>{proposal.title}</strong>
            <span>{proposal.status}</span>
            <p>{proposal.body}</p>
            <div>
              <button
                className="button button-secondary"
                type="button"
                disabled={mutatingMemory !== ""}
                onClick={() => resolveMemory(proposal.proposal_id, "approve")}
              >
                Approve
              </button>
              <button
                className="button button-danger"
                type="button"
                disabled={mutatingMemory !== ""}
                onClick={() => resolveMemory(proposal.proposal_id, "reject")}
              >
                Reject
              </button>
            </div>
          </div>
        ))}
        {memoryItems.slice(0, 3).map((item) => (
          <div className="approval-card" key={item.context_id}>
            <strong>{item.title}</strong>
            <span>approved</span>
            <p>{item.body}</p>
            <div>
              <button
                className="button button-danger"
                type="button"
                disabled={mutatingMemory !== ""}
                onClick={() => deleteMemoryItem(item.context_id)}
              >
                Delete
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
