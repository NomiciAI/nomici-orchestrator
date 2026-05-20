import type { ConsoleState } from "../../hooks/useChatWorkspace";
import {
  eventOutput,
  formatTime,
  humanOutput,
  taskRoleLabel,
  taskTone,
} from "../../lib/format";
import { RoleTimeline } from "./RoleTimeline";
import { WorkspaceLists } from "./WorkspaceLists";

export function WorkspacePanel({ state }: { state: ConsoleState }) {
  const {
    activeRunId,
    runStatus,
    activeRouteDecision: routeDecision,
    sessionDetail,
    traceEvents,
    workspaceTasks: tasks,
    workspaceUploads: uploads,
    workspaceArtifacts: artifacts,
    workspaceToolCalls: toolCalls,
    overview,
    memoryProposals,
    memoryItems,
    artifactContent,
    artifactRevisions,
    artifactMutation,
    planArtifact: currentPlanArtifact,
    sessionNeedsPlanReview,
    planRevision,
    setPlanRevision,
    uploadFile,
    setUploadFile,
    workspaceError,
    workspaceMutation,
    mutatingApproval,
    mutatingMemory,
    clarificationAnswer,
    setClarificationAnswer,
    approvePlan,
    revisePlan,
    uploadInput,
    cancelSession,
    resolveApproval,
    resolveMemory,
    deleteMemoryItem,
    submitClarification,
    resolveBlockedAction,
    inspectArtifact,
    downloadArtifact,
  } = state;
  const approvals = overview.pending_approvals;
  const planArtifact = sessionNeedsPlanReview ? currentPlanArtifact : undefined;
  const latestOutput = humanOutput(traceEvents);
  const decision =
    routeDecision ?? sessionDetail?.session.metadata?.route_decision ?? null;
  const openBlockedActions = (sessionDetail?.blocked_actions ?? []).filter(
    (action) => action.status === "open",
  );
  return (
    <section className="run-workspace" aria-label="Current workspace">
      <div className="run-header">
        <div>
          <p className="eyebrow">Workspace</p>
          <h2>{sessionDetail?.session.title || "No active run"}</h2>
          <p>{activeRunId || "Send a message to start"}</p>
        </div>
        <span
          className={`tag ${
            runStatus === "failed"
              ? "tag-danger"
              : runStatus === "running"
                ? "tag-attention"
                : ""
          }`}
        >
          {sessionDetail?.session.status || runStatus}
        </span>
      </div>

      {decision ? (
        <div className="route-decision">
          <div className="mini-heading no-border">
            <strong>Route decision</strong>
            <span>{decision.mode}</span>
          </div>
          <div className="route-grid">
            <span>Complexity</span>
            <strong>{decision.complexity}</strong>
            <span>Agent</span>
            <strong>{decision.recommended_agent_id || "auto"}</strong>
            <span>Plan review</span>
            <strong>{decision.needs_plan_review ? "required" : "auto"}</strong>
            <span>Risk</span>
            <strong>{decision.risk || "medium"}</strong>
            <span>Confidence</span>
            <strong>
              {decision.confidence
                ? `${Math.round(decision.confidence * 100)}%`
                : "heuristic"}
            </strong>
          </div>
          {decision.rationale ? <p>{decision.rationale}</p> : null}
          {decision.required_tools?.length ? (
            <div className="chip-row">
              {decision.required_tools.map((tool) => (
                <span className="chip" key={tool}>
                  {tool}
                </span>
              ))}
            </div>
          ) : null}
          {decision.required_skills?.length ? (
            <div className="chip-row">
              {decision.required_skills.map((skill) => (
                <span className="chip" key={skill}>
                  {skill}
                </span>
              ))}
            </div>
          ) : null}
          {decision.missing_inputs?.length ? (
            <p>Missing: {decision.missing_inputs.join(", ")}</p>
          ) : null}
        </div>
      ) : null}

      {openBlockedActions.length > 0 ? (
        <div className="blocked-panel">
          <div className="mini-heading no-border">
            <strong>Needs input</strong>
            <span>{openBlockedActions.length}</span>
          </div>
          {openBlockedActions.map((action) => (
            <div className="blocked-card" key={action.blocked_action_id}>
              <div>
                <strong>{action.title}</strong>
                <span>{action.body || action.required_action}</span>
                <small>
                  {action.kind}
                  {action.approval_id ? ` / ${action.approval_id}` : ""}
                  {action.tool_call_id ? ` / ${action.tool_call_id}` : ""}
                </small>
              </div>
              {action.kind === "clarification" ? (
                <div className="clarification-form">
                  <textarea
                    value={clarificationAnswer}
                    onChange={(event) =>
                      setClarificationAnswer(event.target.value)
                    }
                    placeholder="Answer the blocking question"
                  />
                  <button
                    className="button"
                    type="button"
                    disabled={
                      workspaceMutation !== "" ||
                      clarificationAnswer.trim() === ""
                    }
                    onClick={() =>
                      submitClarification(action.blocked_action_id)
                    }
                  >
                    Submit
                  </button>
                </div>
              ) : action.kind === "retry_decision" ||
                action.kind === "tool_risk_review" ? (
                <div className="blocked-actions">
                  <button
                    className="button button-secondary"
                    type="button"
                    disabled={workspaceMutation !== ""}
                    onClick={() =>
                      resolveBlockedAction(action.blocked_action_id, "retry")
                    }
                  >
                    Retry
                  </button>
                  <button
                    className="button button-secondary"
                    type="button"
                    disabled={workspaceMutation !== ""}
                    onClick={() =>
                      resolveBlockedAction(action.blocked_action_id, "skip")
                    }
                  >
                    Skip
                  </button>
                  <button
                    className="button button-danger"
                    type="button"
                    disabled={workspaceMutation !== ""}
                    onClick={() =>
                      resolveBlockedAction(action.blocked_action_id, "stop")
                    }
                  >
                    Stop
                  </button>
                </div>
              ) : null}
            </div>
          ))}
        </div>
      ) : null}

      {tasks.length > 0 ? <RoleTimeline tasks={tasks} /> : null}

      <div className="task-ledger">
        <div className="mini-heading">
          <strong>Task ledger</strong>
          <span>{tasks.length}</span>
        </div>
        {tasks.map((task) => (
          <div className="ledger-row" key={task.task_id}>
            <div>
              <strong>{taskRoleLabel(task)}</strong>
              <span>
                {task.metadata?.summary ||
                  task.metadata?.purpose ||
                  task.blocked_reason ||
                  "pending"}
              </span>
              <small>
                {task.started_at ? formatTime(task.started_at) : "-"} /{" "}
                {task.completed_at
                  ? formatTime(task.completed_at)
                  : task.updated_at
                    ? formatTime(task.updated_at)
                    : "-"}
                {task.artifact_refs?.length
                  ? ` / artifacts ${task.artifact_refs.length}`
                  : ""}
                {task.metadata?.selection_reason
                  ? ` / ${task.metadata.selection_reason}`
                  : ""}
              </small>
            </div>
            <span className={`pill ${taskTone(task.status)}`}>
              {task.status}
            </span>
          </div>
        ))}
        {tasks.length === 0 ? (
          <p className="empty compact-empty">
            Task records appear when a run starts.
          </p>
        ) : null}
        {sessionDetail?.sandbox ? (
          <div className="workspace-roots">
            <span>Workspace</span>
            <code>{sessionDetail.sandbox.workspace_root || "-"}</code>
            <span>Artifacts</span>
            <code>{sessionDetail.sandbox.artifact_root || "-"}</code>
            <span>Sandbox</span>
            <code>
              {sessionDetail.sandbox.provider} /{" "}
              {sessionDetail.sandbox.cleanup_status}
            </code>
          </div>
        ) : null}
      </div>

      {planArtifact ? (
        <div className="plan-review">
          <div>
            <strong>{planArtifact.title}</strong>
            <span>
              {planArtifact.review_state}, revision {planArtifact.revision}
            </span>
          </div>
          <p>{planArtifact.preview || "No plan preview"}</p>
          <textarea
            value={planRevision}
            onChange={(event) => setPlanRevision(event.target.value)}
            placeholder="Revise the plan before approving"
          />
          <div>
            <button
              className="button button-secondary"
              type="button"
              disabled={workspaceMutation !== ""}
              onClick={revisePlan}
            >
              Revise
            </button>
            <button
              className="button"
              type="button"
              disabled={workspaceMutation !== ""}
              onClick={approvePlan}
            >
              Approve
            </button>
          </div>
        </div>
      ) : null}

      <div className="workspace-output">
        <span>Latest output</span>
        <p>{latestOutput || "Output appears here once the run starts."}</p>
      </div>

      <WorkspaceLists state={state} />

      {workspaceError ? (
        <div className="inline-error panel-inline">{workspaceError}</div>
      ) : null}
      {sessionDetail?.session.status === "running" ? (
        <button
          className="button button-danger workspace-cancel"
          type="button"
          onClick={cancelSession}
        >
          Cancel session
        </button>
      ) : null}
    </section>
  );
}
