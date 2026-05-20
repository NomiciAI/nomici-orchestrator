import type { AgentRecord } from "../../api/types";

export function AgentSummaryList({
  agents,
  setDraft,
}: {
  agents: AgentRecord[];
  setDraft: (next: AgentRecord) => void;
}) {
  return (
    <div className="stack">
      {agents.slice(0, 6).map((agent) => (
        <button
          className="list-item list-button"
          key={agent.id}
          type="button"
          onClick={() => setDraft(agent)}
        >
          <div>
            <strong>{agent.name || agent.id}</strong>
            <span>{agent.role || agent.kind}</span>
          </div>
          <span>{agent.model || agent.runtime || "-"}</span>
        </button>
      ))}
    </div>
  );
}
