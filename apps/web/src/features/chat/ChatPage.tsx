import type { ConsoleState } from "../../hooks/useChatWorkspace";
import { toggleListValue } from "../../lib/lists";
import { WorkspacePanel } from "../workspace/WorkspacePanel";

export function ChatPage({ state }: { state: ConsoleState }) {
  return (
    <section
      className={`chat-layout ${state.showChatWorkspace ? "" : "chat-layout-simple"}`}
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
              {message.metadata?.assistant_source ? (
                <small>
                  source: {message.metadata.assistant_source}
                  {message.metadata.model_profile_id
                    ? ` / ${message.metadata.model_profile_id}`
                    : ""}
                </small>
              ) : null}
            </article>
          ))}
          {state.chatDetail ? null : (
            <div className="empty-chat">
              <p className="eyebrow">New Chat</p>
              <h2>Ask anything, or hand off a larger task.</h2>
              <div className="starter-grid">
                {[
                  "Inspect this repo and suggest the next product improvement",
                  "Create a reusable research agent",
                  "Plan a long-horizon implementation run",
                ].map((prompt) => (
                  <button
                    key={prompt}
                    type="button"
                    onClick={() => state.setMessageText(prompt)}
                  >
                    {prompt}
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>
        {state.chatSuggestions.length > 0 ? (
          <div className="suggestion-row" aria-label="Suggested next steps">
            {state.chatSuggestions.slice(0, 3).map((suggestion) => (
              <button
                key={suggestion}
                type="button"
                onClick={() => state.setMessageText(suggestion)}
              >
                {suggestion}
              </button>
            ))}
          </div>
        ) : null}
        <form className="composer" onSubmit={state.sendMessage}>
          {!state.hasConfiguredModel ? (
            <div className="inline-warning">
              Configure a model in Settings or run <code>nomici setup</code>{" "}
              before sending a chat message.
            </div>
          ) : null}
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
                {state.skillCatalog
                  .filter((skill) => skill.enabled !== false)
                  .map((skill) => (
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
                !state.hasConfiguredModel ||
                state.messageText.trim() === ""
              }
            >
              {state.runStatus === "starting" ? "Starting" : "Send"}
            </button>
          </div>
          <div className="composer-support">
            <div className="model-connection">
              <span>Using</span>
              <strong>{state.activeModelLabel}</strong>
              {state.tokenUsage.total > 0 ? (
                <small>
                  tokens {state.tokenUsage.total.toLocaleString()} · input{" "}
                  {state.tokenUsage.input.toLocaleString()} / output{" "}
                  {state.tokenUsage.output.toLocaleString()}
                </small>
              ) : null}
            </div>
            {state.chatDetail ? (
              <div className="chat-actions">
                <button
                  className="button button-ghost"
                  type="button"
                  onClick={state.exportChat}
                >
                  Export
                </button>
                <button
                  className="button button-ghost"
                  type="button"
                  onClick={state.draftAgentFromChat}
                >
                  Draft agent from chat
                </button>
              </div>
            ) : null}
          </div>
          {state.runError ? (
            <div className="inline-error">{state.runError}</div>
          ) : null}
        </form>
      </section>
      {state.showChatWorkspace ? (
        <WorkspacePanel state={state} compact />
      ) : null}
    </section>
  );
}
