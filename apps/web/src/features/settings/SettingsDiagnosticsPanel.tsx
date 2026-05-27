import type { ConsoleState } from "../../hooks/useChatWorkspace";

export function SettingsDiagnosticsPanel({ state }: { state: ConsoleState }) {
  return (
    <section className="panel" aria-label="Diagnostics">
      <div className="panel-heading">
        <div>
          <h2>Diagnostics</h2>
          <p>Feature readiness gates product actions in Console</p>
        </div>
        <span className="tag">{state.featureReadiness.length}</span>
      </div>
      <div className="stack">
        {state.featureReadiness.map((item) => (
          <div className="list-item" key={item.id}>
            <div>
              <strong>{item.label}</strong>
              <span>{item.reason || item.id}</span>
            </div>
            <span
              className={`pill ${
                item.status === "works"
                  ? "task-completed"
                  : item.status === "diagnostic"
                    ? "task-running"
                    : "task-failed"
              }`}
            >
              {item.status}
            </span>
          </div>
        ))}
        {state.featureReadiness.length === 0 ? (
          <p className="empty compact-empty">No readiness diagnostics loaded.</p>
        ) : null}
      </div>
    </section>
  );
}
