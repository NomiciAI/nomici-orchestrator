import type { ConsoleState } from "../../hooks/useChatWorkspace";

export function SettingsMemoryPanel({ state }: { state: ConsoleState }) {
  return (
    <>
      <section className="panel" aria-label="Memory proposals">
        <div className="panel-heading">
          <h2>Memory proposals</h2>
          <span className="tag">{state.memoryProposals.length}</span>
        </div>
        <div className="stack">
          {state.memoryProposals.map((proposal) => (
            <div className="approval-card" key={proposal.proposal_id}>
              <strong>{proposal.title}</strong>
              <span>{proposal.status}</span>
              <p>{proposal.body}</p>
              <div>
                <button
                  className="button button-secondary"
                  type="button"
                  disabled={state.mutatingMemory !== ""}
                  onClick={() =>
                    state.resolveMemory(proposal.proposal_id, "approve")
                  }
                >
                  Approve
                </button>
                <button
                  className="button button-danger"
                  type="button"
                  disabled={state.mutatingMemory !== ""}
                  onClick={() =>
                    state.resolveMemory(proposal.proposal_id, "reject")
                  }
                >
                  Reject
                </button>
              </div>
            </div>
          ))}
          {state.memoryProposals.length === 0 ? (
            <p className="empty compact-empty">No pending memory proposals.</p>
          ) : null}
        </div>
      </section>
      <section className="panel" aria-label="Approved memory">
        <div className="panel-heading">
          <h2>Approved memory</h2>
          <span className="tag">{state.memoryItems.length}</span>
        </div>
        <div className="stack">
          {state.memoryItems.map((item) => (
            <div className="approval-card" key={item.context_id}>
              <strong>{item.title}</strong>
              <span>{item.tags?.join(", ") || "project"}</span>
              <p>{item.body}</p>
              <div>
                <button
                  className="button button-danger"
                  type="button"
                  disabled={state.mutatingMemory !== ""}
                  onClick={() => state.deleteMemoryItem(item.context_id)}
                >
                  Delete
                </button>
              </div>
            </div>
          ))}
          {state.memoryItems.length === 0 ? (
            <p className="empty compact-empty">No approved memories yet.</p>
          ) : null}
        </div>
      </section>
    </>
  );
}
