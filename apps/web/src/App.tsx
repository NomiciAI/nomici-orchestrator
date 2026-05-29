import "./styles/index.css";
import { ChatPage } from "./features/chat/ChatPage";
import { AgentsPage } from "./features/settings/AgentsPage";
import { OrchestratePage } from "./features/orchestrate/OrchestratePage";
import { RunsPage } from "./features/orchestrate/RunsPage";
import { SettingsPage } from "./features/settings/SettingsPage";
import { useChatWorkspace } from "./hooks/useChatWorkspace";
import { viewTitle } from "./lib/format";

export function App() {
  const state = useChatWorkspace();

  return (
    <main className={`shell theme-${state.theme}`}>
      <aside className="sidebar" aria-label="Primary navigation">
        <div className="sidebar-brand">
          <img
            alt="Nomici"
            className="brand-logo"
            src="/logo/logo-solid-black.svg"
          />
          <strong>Nomici</strong>
        </div>
        <button
          className="nav-button nav-primary"
          type="button"
          onClick={state.startNewChat}
        >
          New Chat
        </button>
        <button
          className={`nav-button ${state.view === "chat" ? "nav-active" : ""}`}
          type="button"
          onClick={() => state.setView("chat")}
        >
          Chats
        </button>
        <button
          className={`nav-button ${state.view === "agents" ? "nav-active" : ""}`}
          type="button"
          onClick={() => state.setView("agents")}
        >
          Agents
        </button>
        <button
          className={`nav-button ${state.view === "orchestrate" ? "nav-active" : ""}`}
          type="button"
          onClick={() => state.setView("orchestrate")}
        >
          Orchestration
        </button>
        <button
          className={`nav-button ${state.view === "runs" ? "nav-active" : ""}`}
          type="button"
          onClick={() => state.setView("runs")}
        >
          Runs
        </button>
        <div className="chat-list">
          {state.chats.map((chat) => (
            <button
              className={`chat-list-item ${
                state.chatDetail?.thread.chat_id === chat.chat_id
                  ? "nav-active"
                  : ""
              }`}
              key={chat.chat_id}
              type="button"
              onClick={() => void state.selectChat(chat.chat_id)}
            >
              <span>{chat.title}</span>
              <small>{chat.status}</small>
            </button>
          ))}
          {state.chats.length === 0 ? (
            <p className="empty compact-empty">No chats yet</p>
          ) : null}
        </div>
        <button
          className={`nav-button sidebar-settings ${
            state.view === "settings" ? "nav-active" : ""
          }`}
          type="button"
          onClick={() => state.setView("settings")}
        >
          Settings
        </button>
      </aside>

      <section className="main-surface">
        <header className="topbar">
          <div className="brand">
            <div>
              <p className="eyebrow">Nomici Console</p>
              <h1>{viewTitle(state.view)}</h1>
            </div>
          </div>
          <div className="topbar-actions">
            <span
              className={`status ${
                state.overview.gateway.status === "ok" ? "status-ok" : ""
              }`}
            >
              {state.overview.gateway.status}
            </span>
            <button
              aria-label={`Switch to ${state.theme === "dark" ? "light" : "dark"} mode`}
              className="theme-toggle"
              onClick={() =>
                state.setTheme(state.theme === "dark" ? "light" : "dark")
              }
              type="button"
            >
              <span className="theme-toggle-track" aria-hidden="true">
                <span className="theme-toggle-thumb" />
              </span>
              <span>{state.theme === "dark" ? "Dark" : "Light"}</span>
            </button>
            <button
              className="button"
              type="button"
              onClick={() => void state.loadOverview()}
            >
              Refresh
            </button>
          </div>
        </header>

        {state.status === "loading" ? (
          <div className="auth-banner auth-screen auth-status-card">
            <div>
              <strong>Connecting to Gateway</strong>
              <span>
                Nomici is loading the local Console state. If this does not
                change, restart `nomici dev` and refresh this page.
              </span>
            </div>
          </div>
        ) : null}

        {state.status === "auth" ? (
          <form
            className="auth-banner auth-screen"
            onSubmit={state.submitToken}
          >
            <div>
              <strong>Gateway token required</strong>
              <span>Run this in the workspace that started `nomici dev`.</span>
              <code>nomici gateway token show</code>
              {state.error ? (
                <span className="auth-error">{state.error}</span>
              ) : null}
            </div>
            <input
              aria-label="Gateway token"
              autoComplete="off"
              onChange={(event) => state.setTokenInput(event.target.value)}
              placeholder="Gateway token"
              type="password"
              value={state.tokenInput}
            />
            <div className="auth-actions">
              <button className="button" type="submit">
                Connect
              </button>
              <button
                className="button button-secondary"
                type="button"
                onClick={() => void state.reconnectLocalGateway()}
              >
                Reconnect local Gateway
              </button>
            </div>
          </form>
        ) : null}
        {state.status === "failed" ? (
          <div className="auth-banner auth-screen auth-status-card">
            <div>
              <strong>Gateway unavailable</strong>
              <span>{state.error}</span>
              <code>nomici dev</code>
            </div>
            <button
              className="button"
              type="button"
              onClick={() => void state.loadOverview()}
            >
              Retry
            </button>
          </div>
        ) : null}
        {state.warnings.map((warning) => (
          <div className="banner" key={warning}>
            {warning}
          </div>
        ))}

        {state.isAuthenticated && state.view === "chat" ? (
          <ChatPage state={state} />
        ) : null}
        {state.isAuthenticated && state.view === "agents" ? (
          <AgentsPage state={state} />
        ) : null}
        {state.isAuthenticated && state.view === "orchestrate" ? (
          <OrchestratePage state={state} />
        ) : null}
        {state.isAuthenticated && state.view === "runs" ? (
          <RunsPage state={state} />
        ) : null}
        {state.isAuthenticated && state.view === "settings" ? (
          <SettingsPage state={state} />
        ) : null}
      </section>
    </main>
  );
}
