import type { ConsoleState } from "../../hooks/useChatWorkspace";
import { formatTime } from "../../lib/format";
import { WorkspacePanel } from "../workspace/WorkspacePanel";

export function RunsPage({ state }: { state: ConsoleState }) {
  return (
    <section className="workspace diagnostics-workspace">
      <section className="panel" aria-label="Run history">
        <div className="panel-heading">
          <h2>Run history</h2>
          <span className="tag">{state.overview.recent_sessions.length}</span>
        </div>
        <div className="stack">
          {state.overview.recent_sessions.map((session) => (
            <button
              className="list-item list-button"
              key={session.session_id}
              type="button"
              onClick={() => {
                state.setActiveRunId(session.run_id);
                state.setActiveSessionId(session.session_id);
                state.setRunStatus("running");
                void state.loadSessionDetail(session.session_id);
              }}
            >
              <div>
                <strong>{session.title || session.session_id}</strong>
                <span>{session.execution_state || session.run_id}</span>
              </div>
              <div className="list-meta">
                <span>{session.status}</span>
                <span>{formatTime(session.updated_at)}</span>
              </div>
            </button>
          ))}
          {state.overview.recent_sessions.length === 0 ? (
            <p className="empty compact-empty">No runs yet.</p>
          ) : null}
        </div>
      </section>

      <section className="panel" aria-label="Review queue">
        <div className="panel-heading">
          <h2>Review queue</h2>
          <span className="tag">{state.reviewQueue.length}</span>
        </div>
        <div className="stack">
          {state.reviewQueue.slice(0, 10).map((action) => (
            <button
              className="list-item list-button"
              key={action.blocked_action_id}
              type="button"
              onClick={() => {
                state.setActiveRunId(action.run_id);
                state.setActiveSessionId(action.session_id);
                void state.loadSessionDetail(action.session_id);
              }}
            >
              <div>
                <strong>{action.title}</strong>
                <span>{action.required_action || action.kind}</span>
              </div>
              <div className="list-meta">
                <span>{action.status}</span>
                <span>{formatTime(action.updated_at)}</span>
              </div>
            </button>
          ))}
          {state.reviewQueue.length === 0 ? (
            <p className="empty compact-empty">No open review items.</p>
          ) : null}
        </div>
      </section>

      {state.hasWorkspaceActivity ? (
        <WorkspacePanel state={state} />
      ) : (
        <section className="panel" aria-label="Run detail">
          <div className="panel-heading">
            <h2>Run detail</h2>
            <span className="tag">idle</span>
          </div>
          <p className="empty compact-empty">
            Select a run to inspect timeline, todos, tools, artifacts, and
            review items.
          </p>
        </section>
      )}

      <section className="panel" aria-label="Timeline">
        <div className="panel-heading">
          <h2>Timeline</h2>
          <span className="tag">{state.sessionTimeline.length}</span>
        </div>
        <div className="timeline-list">
          {state.sessionTimeline.slice(0, 40).map((item) => (
            <div className="timeline-item" key={item.id}>
              <strong>{item.title}</strong>
              <span>
                {item.kind}
                {item.status ? ` / ${item.status}` : ""}
              </span>
              <small>{formatTime(item.time)}</small>
            </div>
          ))}
          {state.sessionTimeline.length === 0 ? (
            <p className="empty compact-empty">No selected run timeline.</p>
          ) : null}
        </div>
      </section>

      <section className="panel" aria-label="Todos">
        <div className="panel-heading">
          <h2>Todos</h2>
          <span className="tag">{state.sessionTodos.length}</span>
        </div>
        <div className="todo-list">
          {state.sessionTodos.map((todo) => (
            <div className="todo-item" key={todo.id}>
              <span className={`todo-dot todo-${todo.status}`} />
              <div>
                <strong>{todo.title}</strong>
                <small>{todo.summary || todo.agent_id || todo.status}</small>
              </div>
            </div>
          ))}
          {state.sessionTodos.length === 0 ? (
            <p className="empty compact-empty">No todos for this run.</p>
          ) : null}
        </div>
      </section>
    </section>
  );
}
