import type { ConsoleState } from "../../hooks/useChatWorkspace";
import { humanOutput, taskRoleLabel } from "../../lib/format";
import { RoleTimeline } from "./RoleTimeline";
import {
  RouteDecisionPanel,
  TaskLedger,
  WorkspaceDetails,
  WorkspaceRoots,
} from "./WorkspaceInternals";
import { WorkspaceLists } from "./WorkspaceLists";

export function WorkspacePanel({
  state,
  compact = false,
}: {
  state: ConsoleState;
  compact?: boolean;
}) {
  const {
    activeRunId,
    runStatus,
    activeRouteDecision: routeDecision,
    sessionDetail,
    traceEvents,
    workspaceTasks: tasks,
    planArtifact: currentPlanArtifact,
    sessionNeedsPlanReview,
    planRevision,
    setPlanRevision,
    workspaceError,
    workspaceMutation,
    clarificationAnswer,
    setClarificationAnswer,
    approvePlan,
    revisePlan,
    cancelSession,
    submitClarification,
    resolveBlockedAction,
  } = state;
  const planArtifact = sessionNeedsPlanReview ? currentPlanArtifact : undefined;
  const latestOutput = humanOutput(traceEvents);
  const decision =
    routeDecision ?? sessionDetail?.session.metadata?.route_decision ?? null;
  const openBlockedActions = (sessionDetail?.blocked_actions ?? []).filter(
    (action) => action.status === "open",
  );
  const currentTask =
    tasks.find((task) => task.status === "running") ??
    tasks.find((task) => task.status === "blocked") ??
    tasks.find((task) => task.status === "pending" || task.status === "queued");
  const hasLatestOutput = latestOutput.trim() !== "";
  const hasRunInternals =
    Boolean(decision) ||
    tasks.length > 0 ||
    Boolean(sessionDetail?.sandbox) ||
    state.workspaceUploads.length > 0 ||
    state.workspaceArtifacts.length > 0 ||
    state.workspaceToolCalls.length > 0 ||
    state.sessionApprovals.length > 0;
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

      {sessionDetail ? (
        <div className="workspace-status-banner">
          <strong>{currentTask ? taskRoleLabel(currentTask) : "Run state"}</strong>
          <span>
            {openBlockedActions[0]?.required_action ||
              openBlockedActions[0]?.title ||
              currentTask?.blocked_reason ||
              currentTask?.metadata?.summary ||
              sessionDetail.session.status}
          </span>
        </div>
      ) : null}

      {!compact && decision ? <RouteDecisionPanel decision={decision} /> : null}

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

      {compact && hasRunInternals ? (
        <WorkspaceDetails
          state={state}
          decision={decision}
          tasks={tasks}
          sessionDetail={sessionDetail}
        />
      ) : null}

      {!compact ? (
        <>
          <TaskLedger tasks={tasks} />
          {sessionDetail?.sandbox ? (
            <WorkspaceRoots sandbox={sessionDetail.sandbox} />
          ) : null}
        </>
      ) : null}

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

      {hasLatestOutput || !compact ? (
        <div className="workspace-output">
          <span>Latest output</span>
          <p>{latestOutput || "Output appears here once the run starts."}</p>
        </div>
      ) : null}

      {!compact ? <WorkspaceLists state={state} /> : null}

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
