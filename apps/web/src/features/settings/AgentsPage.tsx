import type { ConsoleState } from "../../hooks/useChatWorkspace";
import { AgentBuilder } from "./AgentBuilder";

export function AgentsPage({ state }: { state: ConsoleState }) {
  return (
    <section className="workspace diagnostics-workspace">
      <section className="panel" aria-label="Agent templates">
        <div className="panel-heading">
          <div>
            <h2>Agent Studio</h2>
            <p>Create, test, and save reusable project agents</p>
          </div>
          <div className="panel-actions">
            <button
              className="button button-secondary"
              type="button"
              onClick={state.draftAgentFromChat}
            >
              From chat
            </button>
            <button
              className="button"
              type="button"
              onClick={state.resetAgentDraft}
            >
              New agent
            </button>
            <span className="tag">{state.agents.length}</span>
          </div>
        </div>
        <div className="template-grid">
          {state.agentTemplates.map((template) => (
            <button
              className="template-card"
              key={template.id}
              type="button"
              onClick={() => state.applyAgentTemplate(template)}
            >
              <strong>{template.name}</strong>
              <span>{template.description}</span>
              <small>{template.tags?.join(", ") || template.kind}</small>
            </button>
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
        testResult={state.agentTestResult}
        testing={state.settingsMutation === "agent-test"}
        onValidate={() => void state.validateAgentDraft()}
        onTest={() => void state.testAgentDraft()}
        onSave={(event) => void state.saveAgent(event)}
      />
    </section>
  );
}
