import type { ConsoleState } from "../../hooks/useChatWorkspace";
import { toggleListValue } from "../../lib/lists";
import { WorkspacePanel } from "../workspace/WorkspacePanel";

export function ChatPage({ state }: { state: ConsoleState }) {
  return (
    <section
      className={`chat-layout ${state.hasWorkspaceActivity ? "" : "chat-layout-simple"}`}
      aria-label="Chat workspace"
    >
      <section className="chat-main">
        <div className="chat-transcript">
          {(state.chatDetail?.messages ?? []).map((message) => (
            <article
              className={"message message-" + message.role}
              key={message.message_id}
            >
              <span>{message.role}</span>
              <p>{message.content}</p>
            </article>
          ))}
          {state.chatDetail ? null : (
            <div className="empty-chat">
              <p className="eyebrow">New Chat</p>
              <h2>Ask anything, or hand off a larger task.</h2>
            </div>
          )}
        </div>
        <form className="composer" onSubmit={state.sendMessage}>
          <textarea
            rows={5}
            value={state.messageText}
            onChange={(event) => state.setMessageText(event.target.value)}
            placeholder="Message Nomici"
          />
          <div className="composer-controls">
            <details className="advanced-agent-picker">
              <summary>
                Agent: {state.runAgentId === "auto" ? "Auto" : state.runAgentId}
              </summary>
              <select
                value={state.runAgentId}
                onChange={(event) => state.setRunAgentId(event.target.value)}
              >
                <option value="auto">Auto</option>
                {state.agentOptions.map((agent) => (
                  <option
                    value={agent.id}
                    key={agent.id}
                    disabled={!agent.supported}
                  >
                    {agent.id}
                    {agent.supported ? "" : " - " + agent.reason}
                  </option>
                ))}
              </select>
            </details>
            <details className="advanced-agent-picker">
              <summary>
                Skills: {state.selectedSkillIds.length || "Auto"}
              </summary>
              <div className="checkbox-grid composer-skill-grid">
                {state.skillCatalog.map((skill) => (
                  <label key={skill.id}>
                    <input
                      type="checkbox"
                      checked={state.selectedSkillIds.includes(skill.id)}
                      onChange={() =>
                        state.setSelectedSkillIds(
                          toggleListValue(state.selectedSkillIds, skill.id),
                        )
                      }
                    />
                    <span>{skill.name || skill.id}</span>
                  </label>
                ))}
              </div>
            </details>
            <button
              className="button task-submit"
              type="submit"
              disabled={
                state.runStatus === "starting" ||
                state.runStatus === "running" ||
                (state.runAgentId !== "auto" &&
                  !state.selectedAgent?.supported) ||
                state.messageText.trim() === ""
              }
            >
              {state.runStatus === "starting" ? "Starting" : "Send"}
            </button>
          </div>
          {state.runError ? (
            <div className="inline-error">{state.runError}</div>
          ) : null}
        </form>
      </section>
      {state.hasWorkspaceActivity ? <WorkspacePanel state={state} /> : null}
    </section>
  );
}
