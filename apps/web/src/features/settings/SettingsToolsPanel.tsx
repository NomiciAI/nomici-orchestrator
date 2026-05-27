import type { ConsoleState } from "../../hooks/useChatWorkspace";

export function SettingsToolsPanel({ state }: { state: ConsoleState }) {
  return (
    <section className="panel" aria-label="Tool contracts">
      <div className="panel-heading">
        <h2>Tools</h2>
        <span className="tag">{state.toolCatalog.length}</span>
      </div>
      <div className="stack">
        {state.toolCatalog.map((tool) => (
          <div className="list-item" key={tool.id}>
            <div>
              <strong>{tool.id}</strong>
              <span>
                {tool.execution} / {tool.auth} / {tool.mutation_risk}
              </span>
            </div>
            <span
              className={
                tool.execution_status === "executable"
                  ? "pill pill-green"
                  : "pill"
              }
            >
              {tool.execution_status || "diagnostic"}
            </span>
          </div>
        ))}
      </div>
    </section>
  );
}
