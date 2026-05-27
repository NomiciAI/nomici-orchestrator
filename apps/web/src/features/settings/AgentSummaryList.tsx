import type { AgentRecord } from "../../api/types";

export function AgentSummaryList({
  agents,
  setDraft,
  onCopy,
  onSetEnabled,
  onDelete,
}: {
  agents: AgentRecord[];
  setDraft: (next: AgentRecord) => void;
  onCopy?: (agent: AgentRecord) => void;
  onSetEnabled?: (agent: AgentRecord, enabled: boolean) => void;
  onDelete?: (agent: AgentRecord) => void;
}) {
  return (
    <div className="stack">
      {agents.slice(0, 6).map((agent) => (
        <div className="list-item agent-summary-item" key={agent.id}>
          <button
            className="list-button agent-summary-main"
            type="button"
            onClick={() => setDraft(agent)}
          >
            <div>
              <strong>{agent.name || agent.id}</strong>
              <span>{agent.role || agent.kind}</span>
            </div>
            <span>{agent.model || agent.runtime || "-"}</span>
          </button>
          <div className="agent-summary-meta">
            <span className="tag">{agent.source || "project"}</span>
            {agent.source === "built_in" && onCopy ? (
              <button
                className="button button-ghost"
                type="button"
                onClick={() => onCopy(agent)}
              >
                Copy to project
              </button>
            ) : null}
            {agent.source !== "built_in" && onSetEnabled ? (
              <button
                className="button button-ghost"
                type="button"
                onClick={() => onSetEnabled(agent, agent.disabled === true)}
              >
                {agent.disabled ? "Enable" : "Disable"}
              </button>
            ) : null}
            {agent.source !== "built_in" && onDelete ? (
              <button
                className="button button-ghost"
                type="button"
                onClick={() => onDelete(agent)}
              >
                Delete
              </button>
            ) : null}
          </div>
        </div>
      ))}
    </div>
  );
}
