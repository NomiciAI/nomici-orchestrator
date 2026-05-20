import type { ConsoleState } from "../../hooks/useChatWorkspace";
import { AgentBuilder } from "./AgentBuilder";

export function SettingsPage({ state }: { state: ConsoleState }) {
  return (
    <section className="workspace diagnostics-workspace">
      <section className="panel" aria-label="Providers">
        <div className="panel-heading">
          <h2>Provider catalog</h2>
          <span className="tag">{state.providerCatalog.length}</span>
        </div>
        <div className="table">
          <div className="table-row table-head">
            <span>Provider</span>
            <span>Adapter</span>
            <span>Models</span>
            <span>Ready</span>
          </div>
          {state.providerCatalog.map((provider) => (
            <div className="table-row" key={provider.id}>
              <span>{provider.name}</span>
              <span>{provider.adapter_kind}</span>
              <span>{provider.catalog_mode}</span>
              <span>{provider.available ? "yes" : "no"}</span>
            </div>
          ))}
        </div>
      </section>
      <section className="panel" aria-label="Model profiles">
        <div className="panel-heading">
          <h2>Configured models</h2>
          <span className="tag">{state.overview.models.length}</span>
        </div>
        <div className="table">
          <div className="table-row table-head">
            <span>Name</span>
            <span>Kind</span>
            <span>Model</span>
            <span>Secret</span>
          </div>
          {state.overview.models.map((model) => (
            <div className="table-row" key={model.id}>
              <span>{model.name}</span>
              <span>{model.kind}</span>
              <span>{model.model}</span>
              <span>{model.api_key_env || "-"}</span>
            </div>
          ))}
        </div>
      </section>
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
      <AgentBuilder
        agents={state.agents}
        models={state.overview.models}
        graphSnapshot={state.overview.graph_snapshot}
        toolCatalog={state.toolCatalog}
        skillCatalog={state.skillCatalog}
        draft={state.agentDraft}
        setDraft={state.setAgentDraft}
        saving={state.settingsMutation === "agent"}
        validating={state.settingsMutation === "agent-validate"}
        validation={state.agentValidation}
        onValidate={() => void state.validateAgentDraft()}
        onSave={(event) => void state.saveAgent(event)}
      />
    </section>
  );
}
