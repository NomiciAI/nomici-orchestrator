import type { ConsoleState } from "../../hooks/useChatWorkspace";

export function SettingsToolsPanel({ state }: { state: ConsoleState }) {
  return (
    <section className="panel" aria-label="Tool contracts">
      <div className="panel-heading">
        <h2>Tools</h2>
        <span className="tag">{state.overview.tools.length}</span>
      </div>
      <div className="stack">
        {state.overview.tools.map((tool) => (
          <div className="list-item" key={tool.id}>
            <div>
              <strong>{tool.id}</strong>
              <span>
                {tool.provider} / {tool.mode} / {tool.execution}
              </span>
            </div>
            <span className="pill pill-green">{tool.status}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
