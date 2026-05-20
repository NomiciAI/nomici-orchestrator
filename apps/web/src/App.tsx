import { type FormEvent, useEffect, useMemo, useState } from "react";
import "./styles.css";

type Theme = "dark" | "light";
type View = "chat" | "orchestrate" | "settings";

type ApiEnvelope<T> = {
  data: T;
  warnings: string[];
  request_id: string;
};

type ApiErrorEnvelope = {
  error?: {
    code: string;
    message: string;
    remediation?: string;
  };
};

type ProviderProfile = {
  id: string;
  name: string;
  kind: string;
  base_url: string;
  model: string;
  api_key_env: string;
  capabilities?: Record<string, string>;
};

type ProviderDefinition = {
  id: string;
  name: string;
  adapter_kind: string;
  description: string;
  auth_mode: string;
  catalog_mode: string;
  default_base_url: string;
  default_api_key_env?: string;
  available: boolean;
  availability_message?: string;
};

type GraphSnapshot = {
  snapshot_id: string;
  project_id: string;
  ir: {
    models: Record<string, { id: string; kind: string; model: string }>;
    runtimes?: Record<string, { id: string; kind: string; workspace?: string }>;
    agents: Record<
      string,
      { id: string; kind: string; model?: string; runtime?: string }
    >;
    edges: Array<{ id: string; from: string; to: string; mode: string }>;
  };
};

type ToolStatus = {
  id: string;
  kind: string;
  provider: string;
  mode: string;
  status: string;
  auth: string;
  execution: string;
};

type RunSession = {
  session_id: string;
  run_id: string;
  project_id: string;
  graph_snapshot_id: string;
  title: string;
  source_channel: string;
  status: string;
  started_at: string;
  updated_at: string;
  completed_at?: string;
};

type RunTask = {
  task_id: string;
  run_id: string;
  agent_id: string;
  runtime_id?: string;
  status: string;
  artifact_refs?: string[];
  started_at: string;
  updated_at: string;
  metadata?: {
    role_id?: string;
    purpose?: string;
    summary?: string;
    output_preview?: string;
    failure_reason?: string;
  };
};

type SandboxRecord = {
  sandbox_id: string;
  provider: string;
  mode: string;
  status: string;
  workspace_root?: string;
  artifact_root?: string;
  cleanup_status: string;
};

type UploadRecord = {
  upload_id: string;
  filename: string;
  path: string;
  size_bytes: number;
  status: string;
  created_at: string;
};

type ArtifactRecord = {
  artifact_id: string;
  task_id?: string;
  type: string;
  title: string;
  revision: number;
  review_state: string;
  preview?: string;
  path?: string;
  updated_at: string;
};

type RunSessionDetail = {
  session: RunSession;
  tasks: RunTask[];
  sandbox?: SandboxRecord;
  uploads?: UploadRecord[];
  artifacts?: ArtifactRecord[];
};

type TraceEvent = {
  event_id: string;
  run_id: string;
  sequence: number;
  type: string;
  time: string;
  node_id?: string;
  runtime_id?: string;
  payload?: Record<string, unknown>;
  redactions?: string[];
  metadata?: Record<string, unknown>;
};

type Approval = {
  approval_id: string;
  run_id: string;
  status: string;
  risk: string;
  summary: string;
  requested_by_agent?: string;
};

type ChatThread = {
  chat_id: string;
  title: string;
  status: string;
  updated_at: string;
};

type ChatMessage = {
  message_id: string;
  chat_id: string;
  role: string;
  content: string;
  run_id?: string;
  session_id?: string;
  created_at: string;
};

type ChatDetail = {
  thread: ChatThread;
  messages: ChatMessage[];
};

type ChatMessageResponse = {
  message: ChatMessage;
  run?: {
    run_id: string;
    status: string;
    agent_id: string;
    session_id?: string;
  };
};

type Overview = {
  gateway: {
    status: string;
    service: string;
    version: string;
  };
  counts: {
    models: number;
    tools: number;
    packs_installed: number;
    agents: number;
    runtimes: number;
    runs: number;
    pending_approvals: number;
  };
  models: ProviderProfile[];
  tools: ToolStatus[];
  graph_snapshot?: GraphSnapshot;
  recent_sessions: RunSession[];
  latest_trace: TraceEvent[];
  pending_approvals: Approval[];
};

const emptyOverview: Overview = {
  gateway: { status: "unknown", service: "nomici-gateway", version: "dev" },
  counts: {
    models: 0,
    tools: 0,
    packs_installed: 0,
    agents: 0,
    runtimes: 0,
    runs: 0,
    pending_approvals: 0,
  },
  models: [],
  tools: [],
  recent_sessions: [],
  latest_trace: [],
  pending_approvals: [],
};

export function App() {
  const [gatewayToken, setGatewayToken] = useState(
    () => window.localStorage.getItem("nomici.gateway.token") ?? "",
  );
  const [tokenInput, setTokenInput] = useState(gatewayToken);
  const [theme, setTheme] = useState<Theme>(() => readTheme());
  const [view, setView] = useState<View>("chat");
  const [overview, setOverview] = useState<Overview>(emptyOverview);
  const [providerCatalog, setProviderCatalog] = useState<ProviderDefinition[]>(
    [],
  );
  const [chats, setChats] = useState<ChatThread[]>([]);
  const [chatDetail, setChatDetail] = useState<ChatDetail | null>(null);
  const [warnings, setWarnings] = useState<string[]>([]);
  const [status, setStatus] = useState<"loading" | "ready" | "failed" | "auth">(
    "loading",
  );
  const [error, setError] = useState("");
  const [runAgentId, setRunAgentId] = useState("");
  const [messageText, setMessageText] = useState("");
  const [activeRunId, setActiveRunId] = useState("");
  const [activeSessionId, setActiveSessionId] = useState("");
  const [sessionDetail, setSessionDetail] = useState<RunSessionDetail | null>(
    null,
  );
  const [runEvents, setRunEvents] = useState<TraceEvent[]>([]);
  const [runStatus, setRunStatus] = useState<
    "idle" | "starting" | "running" | "completed" | "failed"
  >("idle");
  const [runError, setRunError] = useState("");
  const [workspaceError, setWorkspaceError] = useState("");
  const [planRevision, setPlanRevision] = useState("");
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [workspaceMutation, setWorkspaceMutation] = useState("");
  const [mutatingApproval, setMutatingApproval] = useState("");

  const isAuthenticated = status !== "auth";
  const agentOptions = useMemo(
    () => buildAgentOptions(overview.graph_snapshot),
    [overview.graph_snapshot],
  );
  const selectedAgent = agentOptions.find((agent) => agent.id === runAgentId);
  const traceEvents = runEvents.length > 0 ? runEvents : overview.latest_trace;
  const workspaceTasks = sessionDetail?.tasks ?? [];
  const workspaceUploads = sessionDetail?.uploads ?? [];
  const workspaceArtifacts = sessionDetail?.artifacts ?? [];
  const planArtifact = workspaceArtifacts.find(
    (artifact) => artifact.type === "plan",
  );
  const sessionNeedsPlanReview =
    sessionDetail?.session.status === "plan_review" && planArtifact;

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    window.localStorage.setItem("nomici.console.theme", theme);
  }, [theme]);

  useEffect(() => {
    void loadOverview();
  }, []);

  useEffect(() => {
    if (runAgentId === "" && agentOptions.length > 0) {
      const firstRunnable = agentOptions.find((agent) => agent.supported);
      setRunAgentId(firstRunnable?.id ?? agentOptions[0].id);
    }
  }, [agentOptions, runAgentId]);

  useEffect(() => {
    if (!activeRunId || runStatus !== "running") {
      return;
    }
    const state = { cancelled: false };
    if (activeSessionId) {
      void streamSessionEvents(activeSessionId, state);
    }
    const timer = window.setInterval(() => {
      void pollRunEvents(activeRunId, state);
    }, activeSessionId ? 2500 : 1200);
    void pollRunEvents(activeRunId, state);
    return () => {
      state.cancelled = true;
      window.clearInterval(timer);
    };
  }, [activeRunId, activeSessionId, runEvents, runStatus]);

  async function loadOverview(nextToken = gatewayToken) {
    setStatus("loading");
    setError("");
    try {
      const nextOverview = await apiRequest<Overview>(
        "/api/console/overview",
        {},
        nextToken,
      );
      setOverview(normalizeOverview(nextOverview));
      const [nextChats, catalog] = await Promise.all([
        apiRequest<ChatThread[]>("/api/chats?limit=50", {}, nextToken),
        apiRequest<ProviderDefinition[]>("/api/provider-catalog", {}, nextToken),
      ]);
      setChats(nextChats ?? []);
      setProviderCatalog(catalog ?? []);
      setStatus("ready");
    } catch (loadError) {
      const message =
        loadError instanceof Error ? loadError.message : "Gateway unavailable";
      if (message.includes("token")) {
        setStatus("auth");
      } else {
        setStatus("failed");
      }
      setError(message);
    }
  }

  async function apiRequest<T>(
    path: string,
    init: RequestInit = {},
    tokenOverride?: string,
  ): Promise<T> {
    const headers: Record<string, string> = { Accept: "application/json" };
    if (init.body && !(init.body instanceof FormData)) {
      headers["Content-Type"] = "application/json";
    }
    const token = tokenOverride ?? gatewayToken;
    if (token.trim() !== "") {
      headers.Authorization = `Bearer ${token.trim()}`;
    }
    const response = await fetch(path, {
      ...init,
      headers: { ...headers, ...init.headers },
    });
    if (response.status === 401) {
      throw new Error("Gateway token did not match this Gateway");
    }
    const payload = (await response.json()) as ApiEnvelope<T> &
      ApiErrorEnvelope;
    if (!response.ok) {
      throw new Error(
        payload.error?.message ?? `Gateway API returned ${response.status}`,
      );
    }
    if (path === "/api/console/overview") {
      setWarnings(payload.warnings ?? []);
    }
    return payload.data;
  }

  async function submitToken(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const nextToken = tokenInput.trim();
    if (nextToken === "") {
      window.localStorage.removeItem("nomici.gateway.token");
    } else {
      window.localStorage.setItem("nomici.gateway.token", nextToken);
    }
    setGatewayToken(nextToken);
    await loadOverview(nextToken);
  }

  async function selectChat(chatID: string) {
    setView("chat");
    const detail = await apiRequest<ChatDetail>(
      `/api/chats/${encodeURIComponent(chatID)}`,
    );
    setChatDetail(detail);
    const lastRun = [...detail.messages].reverse().find((message) => message.run_id);
    if (lastRun?.run_id) {
      setActiveRunId(lastRun.run_id);
      setActiveSessionId(lastRun.session_id ?? "");
      setRunStatus("running");
      setRunEvents([]);
      if (lastRun.session_id) {
        await loadSessionDetail(lastRun.session_id);
      }
    }
  }

  async function sendMessage(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedAgent?.supported) {
      setRunError(selectedAgent?.reason ?? "Choose a supported entrypoint.");
      return;
    }
    const content = messageText.trim();
    if (content === "") {
      return;
    }
    setRunStatus("starting");
    setRunError("");
    setRunEvents([]);
    try {
      const path = chatDetail
        ? `/api/chats/${encodeURIComponent(chatDetail.thread.chat_id)}/messages`
        : "/api/chats";
      const response = await apiRequest<ChatMessageResponse>(path, {
        method: "POST",
        body: JSON.stringify({
          agent_id: selectedAgent.id,
          prompt: content,
          content,
        }),
      });
      setMessageText("");
      if (response.run) {
        setActiveRunId(response.run.run_id);
        setActiveSessionId(response.run.session_id ?? "");
        setRunStatus("running");
        if (response.run.session_id) {
          await loadSessionDetail(response.run.session_id);
        }
      }
      await loadChatsAndActive(response.message.chat_id);
    } catch (startError) {
      setRunStatus("failed");
      setRunError(
        startError instanceof Error
          ? startError.message
          : "Message could not start a run",
      );
    }
  }

  async function loadChatsAndActive(chatID: string) {
    const [nextChats, detail] = await Promise.all([
      apiRequest<ChatThread[]>("/api/chats?limit=50"),
      apiRequest<ChatDetail>(`/api/chats/${encodeURIComponent(chatID)}`),
    ]);
    setChats(nextChats ?? []);
    setChatDetail(detail);
  }

  async function loadSessionDetail(sessionId: string) {
    if (!sessionId) {
      return;
    }
    const detail = await apiRequest<RunSessionDetail>(
      `/api/sessions/${encodeURIComponent(sessionId)}`,
    );
    setSessionDetail(detail);
  }

  async function pollRunEvents(runId: string, state: { cancelled: boolean }) {
    const lastSequence = runEvents.reduce(
      (max, event) => Math.max(max, event.sequence),
      0,
    );
    try {
      const events = await apiRequest<TraceEvent[]>(
        `/api/runs/${encodeURIComponent(runId)}/events?after_sequence=${lastSequence}`,
      );
      if (state.cancelled || events.length === 0) {
        return;
      }
      setRunEvents((current) => mergeEvents(current, events));
      if (activeSessionId) {
        void loadSessionDetail(activeSessionId);
      }
      const terminal = [...events]
        .reverse()
        .find(
          (event) =>
            event.type === "run.session.completed" ||
            event.type === "run.failed",
        );
      if (terminal) {
        setRunStatus(
          terminal.type === "run.session.completed" ? "completed" : "failed",
        );
        void loadOverview();
      }
    } catch (pollError) {
      if (!state.cancelled) {
        setRunError(
          pollError instanceof Error
            ? pollError.message
            : "Run events could not be loaded",
        );
      }
    }
  }

  async function streamSessionEvents(
    sessionId: string,
    state: { cancelled: boolean },
  ) {
    const lastSequence = runEvents.reduce(
      (max, event) => Math.max(max, event.sequence),
      0,
    );
    const headers: Record<string, string> = { Accept: "text/event-stream" };
    if (gatewayToken.trim() !== "") {
      headers.Authorization = `Bearer ${gatewayToken.trim()}`;
    }
    try {
      const response = await fetch(
        `/api/sessions/${encodeURIComponent(sessionId)}/events?after_sequence=${lastSequence}`,
        { headers },
      );
      if (!response.ok || !response.body) {
        return;
      }
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";
      while (!state.cancelled) {
        const { done, value } = await reader.read();
        if (done) {
          break;
        }
        buffer += decoder.decode(value, { stream: true });
        const parts = buffer.split("\n\n");
        buffer = parts.pop() ?? "";
        for (const part of parts) {
          const dataLine = part
            .split("\n")
            .find((line) => line.startsWith("data: "));
          if (!dataLine) {
            continue;
          }
          const event = JSON.parse(dataLine.slice(6)) as TraceEvent;
          setRunEvents((current) => mergeEvents(current, [event]));
          void loadSessionDetail(sessionId);
          if (event.type === "run.session.completed") {
            setRunStatus("completed");
            void loadOverview();
          }
        }
      }
    } catch {
      return;
    }
  }

  async function approvePlan() {
    if (!activeSessionId || !planArtifact) {
      return;
    }
    setWorkspaceError("");
    setWorkspaceMutation("approve-plan");
    try {
      const detail = await apiRequest<RunSessionDetail>(
        `/api/sessions/${encodeURIComponent(activeSessionId)}/plan/approve`,
        {
          method: "POST",
          body: JSON.stringify({ artifact_id: planArtifact.artifact_id }),
        },
      );
      setSessionDetail(detail);
      setRunStatus("running");
    } catch (approveError) {
      setWorkspaceError(
        approveError instanceof Error
          ? approveError.message
          : "Plan could not be approved",
      );
    } finally {
      setWorkspaceMutation("");
    }
  }

  async function revisePlan() {
    if (!activeSessionId || !planArtifact || planRevision.trim() === "") {
      return;
    }
    setWorkspaceError("");
    setWorkspaceMutation("revise-plan");
    try {
      await apiRequest<ArtifactRecord>(
        `/api/sessions/${encodeURIComponent(activeSessionId)}/plan/revise`,
        {
          method: "POST",
          body: JSON.stringify({
            artifact_id: planArtifact.artifact_id,
            plan: planRevision,
          }),
        },
      );
      setPlanRevision("");
      await loadSessionDetail(activeSessionId);
    } catch (reviseError) {
      setWorkspaceError(
        reviseError instanceof Error
          ? reviseError.message
          : "Plan could not be revised",
      );
    } finally {
      setWorkspaceMutation("");
    }
  }

  async function cancelSession() {
    if (!activeSessionId) {
      return;
    }
    try {
      const detail = await apiRequest<RunSessionDetail>(
        `/api/sessions/${encodeURIComponent(activeSessionId)}/cancel`,
        { method: "POST" },
      );
      setSessionDetail(detail);
      setRunStatus("failed");
      await loadOverview();
    } catch (cancelError) {
      setRunError(
        cancelError instanceof Error
          ? cancelError.message
          : "Session could not be cancelled",
      );
    }
  }

  async function uploadInput() {
    if (!activeSessionId || !uploadFile) {
      return;
    }
    setWorkspaceError("");
    setWorkspaceMutation("upload");
    try {
      const body = new FormData();
      body.append("session_id", activeSessionId);
      body.append("file", uploadFile);
      await apiRequest<UploadRecord>("/api/uploads", { method: "POST", body });
      setUploadFile(null);
      await loadSessionDetail(activeSessionId);
    } catch (uploadError) {
      setWorkspaceError(
        uploadError instanceof Error
          ? uploadError.message
          : "Upload could not be stored",
      );
    } finally {
      setWorkspaceMutation("");
    }
  }

  async function resolveApproval(approvalID: string, action: "grant" | "deny") {
    setMutatingApproval(`${approvalID}:${action}`);
    try {
      await apiRequest<Approval>(
        `/api/approvals/${encodeURIComponent(approvalID)}/${action}`,
        {
          method: "POST",
          body: action === "grant" ? JSON.stringify({ scope: "once" }) : undefined,
        },
      );
      await loadOverview();
    } finally {
      setMutatingApproval("");
    }
  }

  return (
    <main className={`shell theme-${theme}`}>
      <aside className="sidebar" aria-label="Primary navigation">
        <div className="sidebar-brand">
          <img alt="Nomici" className="brand-logo" src="/logo/logo-solid-black.svg" />
          <strong>Nomici</strong>
        </div>
        <button
          className="nav-button nav-primary"
          type="button"
          onClick={() => {
            setView("chat");
            setChatDetail(null);
            setMessageText("");
            setRunStatus("idle");
          }}
        >
          New Chat
        </button>
        <button
          className={`nav-button ${view === "chat" ? "nav-active" : ""}`}
          type="button"
          onClick={() => setView("chat")}
        >
          Chats
        </button>
        <button
          className={`nav-button ${view === "orchestrate" ? "nav-active" : ""}`}
          type="button"
          onClick={() => setView("orchestrate")}
        >
          Orchestrate
        </button>
        <div className="chat-list">
          {chats.map((chat) => (
            <button
              className={`chat-list-item ${
                chatDetail?.thread.chat_id === chat.chat_id ? "nav-active" : ""
              }`}
              key={chat.chat_id}
              type="button"
              onClick={() => void selectChat(chat.chat_id)}
            >
              <span>{chat.title}</span>
              <small>{chat.status}</small>
            </button>
          ))}
          {chats.length === 0 ? <p className="empty compact-empty">No chats yet</p> : null}
        </div>
        <button
          className={`nav-button sidebar-settings ${
            view === "settings" ? "nav-active" : ""
          }`}
          type="button"
          onClick={() => setView("settings")}
        >
          Settings
        </button>
      </aside>

      <section className="main-surface">
        <header className="topbar">
          <div className="brand">
            <div>
              <p className="eyebrow">Nomici Console</p>
              <h1>{viewTitle(view)}</h1>
            </div>
          </div>
          <div className="topbar-actions">
            <span className={`status ${overview.gateway.status === "ok" ? "status-ok" : ""}`}>
              {overview.gateway.status}
            </span>
            <button
              aria-label={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}
              className="theme-toggle"
              onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
              type="button"
            >
              <span className="theme-toggle-track" aria-hidden="true">
                <span className="theme-toggle-thumb" />
              </span>
              <span>{theme === "dark" ? "Dark" : "Light"}</span>
            </button>
            <button className="button" type="button" onClick={() => void loadOverview()}>
              Refresh
            </button>
          </div>
        </header>

        {status === "auth" ? (
          <form className="auth-banner auth-screen" onSubmit={submitToken}>
            <div>
              <strong>Gateway token required</strong>
              <span>Run this in the workspace that started `nomici dev`.</span>
              <code>nomici gateway token show</code>
              {error ? <span className="auth-error">{error}</span> : null}
            </div>
            <input
              aria-label="Gateway token"
              autoComplete="off"
              onChange={(event) => setTokenInput(event.target.value)}
              placeholder="Gateway token"
              type="password"
              value={tokenInput}
            />
            <button className="button" type="submit">
              Connect
            </button>
          </form>
        ) : null}
        {status === "failed" ? <div className="banner banner-error">{error}</div> : null}
        {warnings.map((warning) => (
          <div className="banner" key={warning}>
            {warning}
          </div>
        ))}

        {isAuthenticated && view === "chat" ? (
          <section className="chat-layout" aria-label="Chat workspace">
            <section className="chat-main">
              <div className="chat-transcript">
                {(chatDetail?.messages ?? []).map((message) => (
                  <article className={`message message-${message.role}`} key={message.message_id}>
                    <span>{message.role}</span>
                    <p>{message.content}</p>
                  </article>
                ))}
                {chatDetail ? null : (
                  <div className="empty-chat">
                    <p className="eyebrow">New Chat</p>
                    <h2>What should Nomici work on?</h2>
                  </div>
                )}
              </div>
              <form className="composer" onSubmit={sendMessage}>
                <textarea
                  rows={5}
                  value={messageText}
                  onChange={(event) => setMessageText(event.target.value)}
                  placeholder="Describe the outcome you want delivered"
                />
                <div className="composer-controls">
                  <select value={runAgentId} onChange={(event) => setRunAgentId(event.target.value)}>
                    {agentOptions.map((agent) => (
                      <option value={agent.id} key={agent.id} disabled={!agent.supported}>
                        {agent.id}
                        {agent.supported ? "" : ` - ${agent.reason}`}
                      </option>
                    ))}
                  </select>
                  <button
                    className="button task-submit"
                    type="submit"
                    disabled={
                      runStatus === "starting" ||
                      runStatus === "running" ||
                      !selectedAgent?.supported ||
                      messageText.trim() === ""
                    }
                  >
                    {runStatus === "starting" ? "Starting" : "Send"}
                  </button>
                </div>
                {runError ? <div className="inline-error">{runError}</div> : null}
              </form>
            </section>
            <WorkspacePanel
              activeRunId={activeRunId}
              runStatus={runStatus}
              sessionDetail={sessionDetail}
              traceEvents={traceEvents}
              tasks={workspaceTasks}
              uploads={workspaceUploads}
              artifacts={workspaceArtifacts}
              approvals={overview.pending_approvals}
              planArtifact={sessionNeedsPlanReview ? planArtifact : undefined}
              planRevision={planRevision}
              setPlanRevision={setPlanRevision}
              uploadFile={uploadFile}
              setUploadFile={setUploadFile}
              workspaceError={workspaceError}
              workspaceMutation={workspaceMutation}
              mutatingApproval={mutatingApproval}
              onApprovePlan={() => void approvePlan()}
              onRevisePlan={() => void revisePlan()}
              onUpload={() => void uploadInput()}
              onCancel={() => void cancelSession()}
              onResolveApproval={(approvalID, action) =>
                void resolveApproval(approvalID, action)
              }
            />
          </section>
        ) : null}

        {isAuthenticated && view === "orchestrate" ? (
          <section className="workspace diagnostics-workspace">
            <WorkspacePanel
              activeRunId={activeRunId}
              runStatus={runStatus}
              sessionDetail={sessionDetail}
              traceEvents={traceEvents}
              tasks={workspaceTasks}
              uploads={workspaceUploads}
              artifacts={workspaceArtifacts}
              approvals={overview.pending_approvals}
              planArtifact={sessionNeedsPlanReview ? planArtifact : undefined}
              planRevision={planRevision}
              setPlanRevision={setPlanRevision}
              uploadFile={uploadFile}
              setUploadFile={setUploadFile}
              workspaceError={workspaceError}
              workspaceMutation={workspaceMutation}
              mutatingApproval={mutatingApproval}
              onApprovePlan={() => void approvePlan()}
              onRevisePlan={() => void revisePlan()}
              onUpload={() => void uploadInput()}
              onCancel={() => void cancelSession()}
              onResolveApproval={(approvalID, action) =>
                void resolveApproval(approvalID, action)
              }
            />
            <section className="panel" aria-label="Recent sessions">
              <div className="panel-heading">
                <h2>Sessions</h2>
                <span className="tag">{overview.recent_sessions.length}</span>
              </div>
              <div className="stack">
                {overview.recent_sessions.map((session) => (
                  <button
                    className="list-item list-button"
                    key={session.session_id}
                    type="button"
                    onClick={() => {
                      setActiveRunId(session.run_id);
                      setActiveSessionId(session.session_id);
                      setRunStatus("running");
                      void loadSessionDetail(session.session_id);
                    }}
                  >
                    <div>
                      <strong>{session.title || session.session_id}</strong>
                      <span>{session.run_id}</span>
                    </div>
                    <div className="list-meta">
                      <span>{session.status}</span>
                      <span>{formatTime(session.updated_at)}</span>
                    </div>
                  </button>
                ))}
              </div>
            </section>
          </section>
        ) : null}

        {isAuthenticated && view === "settings" ? (
          <section className="workspace diagnostics-workspace">
            <section className="panel" aria-label="Providers">
              <div className="panel-heading">
                <h2>Provider catalog</h2>
                <span className="tag">{providerCatalog.length}</span>
              </div>
              <div className="table">
                <div className="table-row table-head">
                  <span>Provider</span>
                  <span>Adapter</span>
                  <span>Models</span>
                  <span>Ready</span>
                </div>
                {providerCatalog.map((provider) => (
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
                <span className="tag">{overview.models.length}</span>
              </div>
              <div className="table">
                <div className="table-row table-head">
                  <span>Name</span>
                  <span>Kind</span>
                  <span>Model</span>
                  <span>Secret</span>
                </div>
                {overview.models.map((model) => (
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
                <span className="tag">{overview.tools.length}</span>
              </div>
              <div className="stack">
                {overview.tools.map((tool) => (
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
          </section>
        ) : null}
      </section>
    </main>
  );
}

function WorkspacePanel({
  activeRunId,
  runStatus,
  sessionDetail,
  traceEvents,
  tasks,
  uploads,
  artifacts,
  approvals,
  planArtifact,
  planRevision,
  setPlanRevision,
  uploadFile,
  setUploadFile,
  workspaceError,
  workspaceMutation,
  mutatingApproval,
  onApprovePlan,
  onRevisePlan,
  onUpload,
  onCancel,
  onResolveApproval,
}: {
  activeRunId: string;
  runStatus: string;
  sessionDetail: RunSessionDetail | null;
  traceEvents: TraceEvent[];
  tasks: RunTask[];
  uploads: UploadRecord[];
  artifacts: ArtifactRecord[];
  approvals: Approval[];
  planArtifact?: ArtifactRecord;
  planRevision: string;
  setPlanRevision: (value: string) => void;
  uploadFile: File | null;
  setUploadFile: (value: File | null) => void;
  workspaceError: string;
  workspaceMutation: string;
  mutatingApproval: string;
  onApprovePlan: () => void;
  onRevisePlan: () => void;
  onUpload: () => void;
  onCancel: () => void;
  onResolveApproval: (approvalID: string, action: "grant" | "deny") => void;
}) {
  const latestOutput = humanOutput(traceEvents);
  return (
    <section className="run-workspace" aria-label="Current workspace">
      <div className="run-header">
        <div>
          <p className="eyebrow">Workspace</p>
          <h2>{sessionDetail?.session.title || "No active run"}</h2>
          <p>{activeRunId || "Send a message to start"}</p>
        </div>
        <span
          className={`tag ${
            runStatus === "failed"
              ? "tag-danger"
              : runStatus === "running"
                ? "tag-attention"
                : ""
          }`}
        >
          {sessionDetail?.session.status || runStatus}
        </span>
      </div>

      <div className="task-ledger">
        <div className="mini-heading">
          <strong>Task ledger</strong>
          <span>{tasks.length}</span>
        </div>
        {tasks.map((task) => (
          <div className="ledger-row" key={task.task_id}>
            <div>
              <strong>{taskRoleLabel(task)}</strong>
              <span>{task.metadata?.summary || task.metadata?.purpose || "pending"}</span>
            </div>
            <span className={`pill ${taskTone(task.status)}`}>{task.status}</span>
          </div>
        ))}
        {tasks.length === 0 ? (
          <p className="empty compact-empty">Task records appear when a run starts.</p>
        ) : null}
        {sessionDetail?.sandbox ? (
          <div className="workspace-roots">
            <span>Workspace</span>
            <code>{sessionDetail.sandbox.workspace_root || "-"}</code>
            <span>Artifacts</span>
            <code>{sessionDetail.sandbox.artifact_root || "-"}</code>
            <span>Sandbox</span>
            <code>
              {sessionDetail.sandbox.provider} / {sessionDetail.sandbox.cleanup_status}
            </code>
          </div>
        ) : null}
      </div>

      {planArtifact ? (
        <div className="plan-review">
          <div>
            <strong>{planArtifact.title}</strong>
            <span>
              {planArtifact.review_state}, revision {planArtifact.revision}
            </span>
          </div>
          <p>{planArtifact.preview || "No plan preview"}</p>
          <textarea
            value={planRevision}
            onChange={(event) => setPlanRevision(event.target.value)}
            placeholder="Revise the plan before approving"
          />
          <div>
            <button
              className="button button-secondary"
              type="button"
              disabled={workspaceMutation !== ""}
              onClick={onRevisePlan}
            >
              Revise
            </button>
            <button
              className="button"
              type="button"
              disabled={workspaceMutation !== ""}
              onClick={onApprovePlan}
            >
              Approve
            </button>
          </div>
        </div>
      ) : null}

      <div className="workspace-output">
        <span>Latest output</span>
        <p>{latestOutput || "Output appears here once the run starts."}</p>
      </div>

      <div className="workspace-lists">
        <div>
          <div className="mini-heading">
            <strong>Uploads</strong>
            <span>{uploads.length}</span>
          </div>
          <div className="upload-box">
            <input
              type="file"
              onChange={(event) => setUploadFile(event.target.files?.[0] ?? null)}
            />
            <button
              className="button button-secondary"
              type="button"
              disabled={!uploadFile || !sessionDetail || workspaceMutation !== ""}
              onClick={onUpload}
            >
              Add
            </button>
          </div>
          {uploads.slice(0, 4).map((upload) => (
            <div className="event-row passive-row" key={upload.upload_id}>
              <span>{upload.filename}</span>
              <strong>{upload.status}</strong>
            </div>
          ))}
        </div>
        <div>
          <div className="mini-heading">
            <strong>Artifacts</strong>
            <span>{artifacts.length}</span>
          </div>
          {artifacts.slice(0, 5).map((artifact) => (
            <div className="event-row passive-row" key={artifact.artifact_id}>
              <span>{artifact.type}</span>
              <strong>{artifact.title}</strong>
            </div>
          ))}
        </div>
        <div>
          <div className="mini-heading">
            <strong>Events</strong>
            <span>{traceEvents.length}</span>
          </div>
          {traceEvents.slice(-6).map((event) => (
            <div className="event-row passive-row" key={event.event_id}>
              <span>{event.type}</span>
              <strong>{eventOutput(event) || formatTime(event.time)}</strong>
            </div>
          ))}
        </div>
        <div>
          <div className="mini-heading">
            <strong>Approvals</strong>
            <span>{approvals.length}</span>
          </div>
          {approvals.slice(0, 3).map((approval) => (
            <div className="approval-card" key={approval.approval_id}>
              <strong>{approval.summary}</strong>
              <span>{approval.risk}</span>
              <div>
                <button
                  className="button button-secondary"
                  type="button"
                  disabled={mutatingApproval !== ""}
                  onClick={() => onResolveApproval(approval.approval_id, "grant")}
                >
                  Grant
                </button>
                <button
                  className="button button-danger"
                  type="button"
                  disabled={mutatingApproval !== ""}
                  onClick={() => onResolveApproval(approval.approval_id, "deny")}
                >
                  Deny
                </button>
              </div>
            </div>
          ))}
        </div>
      </div>

      {workspaceError ? <div className="inline-error panel-inline">{workspaceError}</div> : null}
      {sessionDetail?.session.status === "running" ? (
        <button className="button button-danger workspace-cancel" type="button" onClick={onCancel}>
          Cancel session
        </button>
      ) : null}
    </section>
  );
}

type AgentOption = {
  id: string;
  supported: boolean;
  reason: string;
};

function buildAgentOptions(snapshot?: GraphSnapshot): AgentOption[] {
  if (!snapshot) {
    return [];
  }
  return Object.keys(snapshot.ir.agents)
    .sort()
    .map((id) => {
      const agent = snapshot.ir.agents[id];
      const outgoing = snapshot.ir.edges.filter((edge) => edge.from === id);
      if (agent.kind === "gateway_agent" || agent.kind === "model_agent") {
        if (outgoing.length > 0) {
          return {
            id,
            supported: false,
            reason: "model agents with outgoing edges are not executable yet",
          };
        }
        return { id, supported: Boolean(agent.model), reason: agent.model ? "" : "missing model" };
      }
      if (agent.kind !== "external_agent") {
        return { id, supported: false, reason: `${agent.kind} is not executable` };
      }
      if (!agent.runtime) {
        return { id, supported: false, reason: "missing runtime" };
      }
      const runtime = snapshot.ir.runtimes?.[agent.runtime];
      if (!runtime || runtime.kind !== "cli_agent") {
        return { id, supported: false, reason: "runtime is not a cli_agent" };
      }
      if (outgoing.length === 0) {
        return { id, supported: true, reason: "" };
      }
      const chainCheck = checkHandoffChain(snapshot, id);
      return chainCheck.supported
        ? { id, supported: true, reason: "" }
        : { id, supported: false, reason: chainCheck.reason };
    });
}

function checkHandoffChain(
  snapshot: GraphSnapshot,
  startAgentId: string,
): { supported: boolean; reason: string } {
  const visited = new Set<string>([startAgentId]);
  let current = startAgentId;
  for (;;) {
    const outgoing = snapshot.ir.edges.filter((edge) => edge.from === current);
    if (outgoing.length === 0) {
      return { supported: true, reason: "" };
    }
    if (outgoing.length > 1) {
      return { supported: false, reason: "handoff chain has multiple outgoing edges" };
    }
    const edge = outgoing[0];
    if (edge.mode !== "handoff") {
      return { supported: false, reason: "only handoff chains are executable" };
    }
    const target = snapshot.ir.agents[edge.to];
    const targetRuntime = target?.runtime ? snapshot.ir.runtimes?.[target.runtime] : undefined;
    if (target?.kind !== "external_agent" || targetRuntime?.kind !== "cli_agent") {
      return {
        supported: false,
        reason: "handoff target is not a cli_agent external agent",
      };
    }
    if (visited.has(edge.to)) {
      return { supported: false, reason: "handoff chain contains a cycle" };
    }
    visited.add(edge.to);
    current = edge.to;
  }
}

function normalizeOverview(next: Overview): Overview {
  return {
    ...emptyOverview,
    ...next,
    gateway: { ...emptyOverview.gateway, ...next.gateway },
    counts: { ...emptyOverview.counts, ...next.counts },
    models: next.models ?? [],
    tools: next.tools ?? [],
    recent_sessions: next.recent_sessions ?? [],
    latest_trace: next.latest_trace ?? [],
    pending_approvals: next.pending_approvals ?? [],
  };
}

function taskTone(status: string): string {
  switch (status) {
    case "completed":
      return "pill-green";
    case "failed":
    case "cancelled":
      return "pill-red";
    case "running":
    case "waiting_for_approval":
    case "plan_review":
      return "pill-amber";
    default:
      return "";
  }
}

function taskRoleLabel(task: RunTask): string {
  if (task.metadata?.role_id && task.metadata.role_id !== task.agent_id) {
    return `${task.metadata.role_id} / ${task.agent_id}`;
  }
  return task.metadata?.role_id || task.agent_id;
}

function mergeEvents(current: TraceEvent[], next: TraceEvent[]): TraceEvent[] {
  const byID = new Map<string, TraceEvent>();
  for (const event of current) {
    byID.set(event.event_id, event);
  }
  for (const event of next) {
    byID.set(event.event_id, event);
  }
  return [...byID.values()].sort((a, b) => a.sequence - b.sequence);
}

function eventOutput(event: TraceEvent): string {
  const payload = event.payload ?? {};
  for (const key of ["output_preview", "stdout_preview", "stderr_preview", "message", "error"]) {
    const value = payload[key];
    if (typeof value === "string" && value.trim() !== "") {
      return value;
    }
  }
  return "";
}

function humanOutput(events: TraceEvent[]): string {
  for (const event of [...events].reverse()) {
    const output = eventOutput(event);
    if (output) {
      return output;
    }
  }
  return "";
}

function readTheme(): Theme {
  const saved = window.localStorage.getItem("nomici.console.theme");
  if (saved === "light" || saved === "dark") {
    return saved;
  }
  return window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
}

function formatTime(value: string): string {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

function viewTitle(view: View): string {
  switch (view) {
    case "chat":
      return "Chat";
    case "orchestrate":
      return "Orchestrate";
    case "settings":
      return "Settings";
    default:
      return "Chat";
  }
}
