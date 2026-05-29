import type { ConsoleState } from "../../hooks/useChatWorkspace";
import { formatTime, taskRoleLabel, taskTone } from "../../lib/format";
import { WorkspaceLists } from "./WorkspaceLists";

type RouteDecision = NonNullable<ConsoleState["activeRouteDecision"]>;
type SessionDetail = NonNullable<ConsoleState["sessionDetail"]>;

export function WorkspaceDetails({
  state,
  decision,
  tasks,
  sessionDetail,
}: {
  state: ConsoleState;
  decision: RouteDecision | null;
  tasks: ConsoleState["workspaceTasks"];
  sessionDetail: ConsoleState["sessionDetail"];
}) {
  return (
    <details className="workspace-details">
      <summary>Run details</summary>
      {decision ? <RouteDecisionPanel decision={decision} /> : null}
      {tasks.length > 0 ? <TaskLedger tasks={tasks} /> : null}
      {sessionDetail?.sandbox ? (
        <WorkspaceRoots sandbox={sessionDetail.sandbox} />
      ) : null}
      <WorkspaceLists state={state} />
    </details>
  );
}

export function RouteDecisionPanel({
  decision,
}: {
  decision: RouteDecision;
}) {
  return (
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
  );
}

export function TaskLedger({
  tasks,
}: {
  tasks: ConsoleState["workspaceTasks"];
}) {
  return (
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
          <span className={`pill ${taskTone(task.status)}`}>{task.status}</span>
        </div>
      ))}
      {tasks.length === 0 ? (
        <p className="empty compact-empty">
          Task records appear when a run starts.
        </p>
      ) : null}
    </div>
  );
}

export function WorkspaceRoots({
  sandbox,
}: {
  sandbox: NonNullable<SessionDetail["sandbox"]>;
}) {
  return (
    <div className="workspace-roots">
      <span>Workspace</span>
      <code>{sandbox.workspace_root || "-"}</code>
      <span>Artifacts</span>
      <code>{sandbox.artifact_root || "-"}</code>
      <span>Sandbox</span>
      <code>
        {sandbox.provider} / {sandbox.cleanup_status}
      </code>
    </div>
  );
}
