import type { ConsoleState } from "../../hooks/useChatWorkspace";
import { formatTime } from "../../lib/format";
import { WorkspacePanel } from "../workspace/WorkspacePanel";
import { OrchestrateBuilder } from "./OrchestrateBuilder";

export function OrchestratePage({ state }: { state: ConsoleState }) {
  return (
    <section className="workspace diagnostics-workspace">
      <WorkspacePanel state={state} />
      <section className="panel" aria-label="Recent sessions">
        <div className="panel-heading">
          <h2>Sessions</h2>
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
                <span>{session.run_id}</span>
              </div>
              <div className="list-meta">
                <span>{session.status}</span>
                <span>{formatTime(session.updated_at)}</span>
              </div>
            </button>
          ))}
        </div>
      </section>
      <section className="panel" aria-label="Review queue">
        <div className="panel-heading">
          <h2>Review queue</h2>
          <span className="tag">{state.reviewQueue.length}</span>
        </div>
        <div className="stack">
          {state.reviewQueue.slice(0, 8).map((action) => (
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
                <span>{action.kind}</span>
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
      <OrchestrateBuilder
        agents={state.agents}
        orchestration={state.orchestration}
        toolCatalog={state.toolCatalog}
        skillCatalog={state.skillCatalog}
        saving={state.settingsMutation === "orchestration"}
        onSave={(next) => void state.saveOrchestration(next)}
      />
    </section>
  );
}
