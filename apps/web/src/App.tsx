import {
  Background,
  Controls,
  ReactFlow,
  type Edge,
  type Node,
} from "@xyflow/react";
import { type FormEvent, useEffect, useMemo, useState } from "react";
import "@xyflow/react/dist/style.css";
import "./styles.css";

type Theme = "dark" | "light";

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

type PackStatus = {
  manifest: {
    id: string;
    name: string;
    version: string;
    kind: string;
    permissions: {
      filesystem?: {
        read?: string[];
        write?: string[];
      };
      shell?: {
        mode?: string;
      };
    };
    agents?: {
      entrypoints?: string[];
      includes?: string[];
      optional?: string[];
    };
    trust?: {
      level?: string;
    };
  };
  installed: boolean;
  installation?: {
    config_path: string;
    entrypoints: string[];
    installed_at: string;
  };
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

type RuntimeStatus = {
  id: string;
  kind: string;
  workspace?: string;
  trust?: string;
  status: string;
  agents?: string[];
};

type RunSummary = {
  run_id: string;
  event_count: number;
  first_time: string;
  last_time: string;
  last_type: string;
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
  parent_task_id?: string;
  agent_id: string;
  runtime_id?: string;
  status: string;
  context_snapshot_id?: string;
  artifact_refs?: string[];
  approval_refs?: string[];
  started_at: string;
  updated_at: string;
  completed_at?: string;
  metadata?: {
    role_id?: string;
    purpose?: string;
    handoff_mode?: string;
    plan_source?: string;
  };
};

type SandboxRecord = {
  sandbox_id: string;
  run_id: string;
  task_id?: string;
  provider: string;
  mode: string;
  status: string;
  workspace_root?: string;
  artifact_root?: string;
  cleanup_status: string;
};

type RunSessionDetail = {
  session: RunSession;
  tasks: RunTask[];
  sandbox?: SandboxRecord;
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
  scope?: string;
  summary: string;
  requested_by_agent?: string;
};

type Overview = {
  gateway: {
    status: string;
    service: string;
    version: string;
  };
  counts: {
    models: number;
    packs_installed: number;
    agents: number;
    runtimes: number;
    runs: number;
    pending_approvals: number;
  };
  models: ProviderProfile[];
  packs: PackStatus[];
  graph_snapshot?: GraphSnapshot;
  runtimes: RuntimeStatus[];
  recent_runs: RunSummary[];
  recent_sessions: RunSession[];
  latest_trace: TraceEvent[];
  pending_approvals: Approval[];
  unavailable: Array<{ name: string; status: string; reason: string }>;
};

const emptyOverview: Overview = {
  gateway: { status: "unknown", service: "nomici-gateway", version: "dev" },
  counts: {
    models: 0,
    packs_installed: 0,
    agents: 0,
    runtimes: 0,
    runs: 0,
    pending_approvals: 0,
  },
  models: [],
  packs: [],
  runtimes: [],
  recent_runs: [],
  recent_sessions: [],
  latest_trace: [],
  pending_approvals: [],
  unavailable: [],
};

export function App() {
  const [gatewayToken, setGatewayToken] = useState(
    () => window.localStorage.getItem("nomici.gateway.token") ?? "",
  );
  const [theme, setTheme] = useState<Theme>(() => readTheme());
  const [tokenInput, setTokenInput] = useState(gatewayToken);
  const [overview, setOverview] = useState<Overview>(emptyOverview);
  const [warnings, setWarnings] = useState<string[]>([]);
  const [status, setStatus] = useState<"loading" | "ready" | "failed" | "auth">(
    "loading",
  );
  const [error, setError] = useState("");
  const [runAgentId, setRunAgentId] = useState("");
  const [runPrompt, setRunPrompt] = useState("");
  const [runEvents, setRunEvents] = useState<TraceEvent[]>([]);
  const [activeRunId, setActiveRunId] = useState("");
  const [activeSessionId, setActiveSessionId] = useState("");
  const [sessionDetail, setSessionDetail] = useState<RunSessionDetail | null>(
    null,
  );
  const [runStatus, setRunStatus] = useState<
    "idle" | "starting" | "running" | "completed" | "failed"
  >("idle");
  const [runError, setRunError] = useState("");
  const [approvalError, setApprovalError] = useState("");
  const [mutatingApproval, setMutatingApproval] = useState("");
  const [expandedEvents, setExpandedEvents] = useState<Record<string, boolean>>(
    {},
  );
  const isAuthenticated = status !== "auth";

  async function loadOverview(nextToken = gatewayToken) {
    setStatus("loading");
    setError("");
    try {
      const headers: Record<string, string> = { Accept: "application/json" };
      if (nextToken.trim() !== "") {
        headers.Authorization = `Bearer ${nextToken.trim()}`;
      }
      const response = await fetch("/api/console/overview", {
        headers,
      });
      if (response.status === 401) {
        setStatus("auth");
        setError(
          nextToken.trim() === ""
            ? "Gateway token required"
            : "Gateway token did not match this Gateway",
        );
        return;
      }
      if (!response.ok) {
        throw new Error(`Gateway API returned ${response.status}`);
      }
      const envelope = (await response.json()) as ApiEnvelope<Overview>;
      setOverview(normalizeOverview(envelope.data));
      setWarnings(envelope.warnings ?? []);
      setStatus("ready");
    } catch (loadError) {
      setError(
        loadError instanceof Error
          ? loadError.message
          : "Gateway API unavailable",
      );
      setStatus("failed");
    }
  }

  useEffect(() => {
    void loadOverview();
  }, []);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    window.localStorage.setItem("nomici.console.theme", theme);
  }, [theme]);

  function submitToken(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const nextToken = tokenInput.trim();
    if (nextToken === "") {
      window.localStorage.removeItem("nomici.gateway.token");
    } else {
      window.localStorage.setItem("nomici.gateway.token", nextToken);
    }
    setGatewayToken(nextToken);
    void loadOverview(nextToken);
  }

  const flow = useMemo(
    () => buildFlow(overview.graph_snapshot),
    [overview.graph_snapshot],
  );
  const agentOptions = useMemo(
    () => buildAgentOptions(overview.graph_snapshot),
    [overview.graph_snapshot],
  );
  const selectedAgent = agentOptions.find((agent) => agent.id === runAgentId);
  const traceEvents = runEvents.length > 0 ? runEvents : overview.latest_trace;
  const runnableAgents = agentOptions.filter((agent) => agent.supported);
  const blockedAgents = agentOptions.filter((agent) => !agent.supported);
  const latestOutput = humanOutput(traceEvents);
  const runTitle = runPrompt.trim() || "No task started";
  const runStages = buildRunStages(runStatus, traceEvents);
  const workspaceTasks = sessionDetail?.tasks ?? [];

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
    const timer = window.setInterval(() => {
      void pollRunEvents(activeRunId, state);
    }, 1200);
    void pollRunEvents(activeRunId, state);
    return () => {
      state.cancelled = true;
      window.clearInterval(timer);
    };
  }, [activeRunId, activeSessionId, runEvents, runStatus]);

  async function apiRequest<T>(
    path: string,
    init: RequestInit = {},
  ): Promise<T> {
    const headers: Record<string, string> = {
      Accept: "application/json",
      ...(init.body ? { "Content-Type": "application/json" } : {}),
    };
    if (gatewayToken.trim() !== "") {
      headers.Authorization = `Bearer ${gatewayToken.trim()}`;
    }
    const response = await fetch(path, {
      ...init,
      headers: { ...headers, ...init.headers },
    });
    if (response.status === 401) {
      setStatus("auth");
      throw new Error("Gateway token did not match this Gateway");
    }
    const payload = (await response.json()) as ApiEnvelope<T> &
      ApiErrorEnvelope;
    if (!response.ok) {
      throw new Error(
        payload.error?.message ?? `Gateway API returned ${response.status}`,
      );
    }
    return payload.data;
  }

  async function startRun(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedAgent?.supported) {
      setRunError(selectedAgent?.reason ?? "Choose a supported agent.");
      return;
    }
    setRunStatus("starting");
    setRunError("");
    setRunEvents([]);
    try {
      const started = await apiRequest<{
        run_id: string;
        status: string;
        agent_id: string;
        graph_snapshot_id: string;
        session_id?: string;
      }>("/api/runs", {
        method: "POST",
        body: JSON.stringify({ agent_id: selectedAgent.id, prompt: runPrompt }),
      });
      setActiveRunId(started.run_id);
      setActiveSessionId(started.session_id ?? "");
      setRunStatus("running");
      if (started.session_id) {
        void loadSessionDetail(started.session_id);
      }
      void loadOverview();
    } catch (startError) {
      setRunStatus("failed");
      setRunError(
        startError instanceof Error
          ? startError.message
          : "Run could not be started",
      );
    }
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

  async function cancelSession(sessionId: string) {
    setRunError("");
    try {
      const detail = await apiRequest<RunSessionDetail>(
        `/api/sessions/${encodeURIComponent(sessionId)}/cancel`,
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
            event.type === "run.completed" || event.type === "run.failed",
        );
      if (terminal) {
        setRunStatus(
          terminal.type === "run.completed" ? "completed" : "failed",
        );
        void loadOverview();
        if (activeSessionId) {
          void loadSessionDetail(activeSessionId);
        }
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

  async function resolveApproval(
    approvalID: string,
    action: "grant" | "deny",
    scope?: "once" | "run",
  ) {
    setApprovalError("");
    setMutatingApproval(`${approvalID}:${action}:${scope ?? ""}`);
    try {
      await apiRequest<Approval>(
        `/api/approvals/${encodeURIComponent(approvalID)}/${action}`,
        {
          method: "POST",
          body: action === "grant" ? JSON.stringify({ scope }) : undefined,
        },
      );
      await loadOverview();
      if (activeRunId) {
        await pollRunEvents(activeRunId, { cancelled: false });
      }
    } catch (approvalActionError) {
      setApprovalError(
        approvalActionError instanceof Error
          ? approvalActionError.message
          : "Approval update failed",
      );
    } finally {
      setMutatingApproval("");
    }
  }

  return (
    <main className={`shell theme-${theme}`}>
      <header className="topbar">
        <div className="brand">
          <img
            alt="Nomici"
            className="brand-logo"
            src="/logo/logo-solid-black.svg"
          />
          <div>
            <p className="eyebrow">Nomici Console</p>
            <h1>Task Workspace</h1>
          </div>
        </div>
        <div className="topbar-actions">
          <span
            className={`status ${overview.gateway.status === "ok" ? "status-ok" : ""}`}
          >
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
          <button
            className="button"
            type="button"
            onClick={() => void loadOverview()}
          >
            Refresh
          </button>
        </div>
      </header>

      {isAuthenticated ? (
        <section className="task-strip" aria-label="Workspace status">
          <Metric label="Runnable entries" value={runnableAgents.length} />
          <Metric label="Recent runs" value={overview.counts.runs} />
          <Metric
            label="Approvals waiting"
            value={overview.counts.pending_approvals}
            tone="attention"
          />
          <Metric
            label="Installed packs"
            value={overview.counts.packs_installed}
          />
        </section>
      ) : null}

      {status === "failed" ? (
        <div className="banner banner-error">{error}</div>
      ) : null}
      {status === "auth" ? (
        <form className="auth-banner auth-screen" onSubmit={submitToken}>
          <div>
            <strong>Gateway token required</strong>
            <span>
              Run this in the same directory where you started `nomici up`, then
              paste the token here.
            </span>
            <code>nomici gateway token show</code>
            <span>
              Tokens are workspace-specific because each `.nomici` state
              directory has its own Gateway token.
            </span>
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
      {warnings.map((warning) => (
        <div className="banner" key={warning}>
          {warning}
        </div>
      ))}

      {isAuthenticated ? (
        <>
          <section className="task-workspace" aria-label="Run workspace">
            <section className="task-entry" aria-label="Start long task">
              <div className="task-entry-copy">
                <p className="eyebrow">Run workspace</p>
                <h2>Start a long-horizon task</h2>
                <p>
                  Give Nomici one outcome. The run workspace keeps the agent,
                  handoffs, approvals, trace, and output in one place.
                </p>
              </div>
              <form className="task-form" onSubmit={startRun}>
                <label>
                  <span>Task</span>
                  <textarea
                    rows={7}
                    value={runPrompt}
                    onChange={(event) => setRunPrompt(event.target.value)}
                    placeholder="Describe the outcome you want delivered"
                  />
                </label>
                <div className="task-controls">
                  <label>
                    <span>Entrypoint</span>
                    <select
                      value={runAgentId}
                      onChange={(event) => setRunAgentId(event.target.value)}
                    >
                      {agentOptions.map((agent) => (
                        <option
                          value={agent.id}
                          key={agent.id}
                          disabled={!agent.supported}
                        >
                          {agent.id}
                          {agent.supported ? "" : ` - ${agent.reason}`}
                        </option>
                      ))}
                    </select>
                  </label>
                  <button
                    className="button task-submit"
                    type="submit"
                    disabled={
                      runStatus === "starting" ||
                      runStatus === "running" ||
                      !selectedAgent?.supported ||
                      runPrompt.trim() === ""
                    }
                  >
                    {runStatus === "starting" ? "Starting" : "Start task"}
                  </button>
                </div>
                {agentOptions.length === 0 ? (
                  <p className="form-hint">
                    No runnable entrypoint found in this workspace yet.
                  </p>
                ) : selectedAgent && !selectedAgent.supported ? (
                  <p className="form-hint">{selectedAgent.reason}</p>
                ) : null}
                {runError ? (
                  <div className="inline-error">{runError}</div>
                ) : null}
              </form>
            </section>

            <section className="run-workspace" aria-label="Current run">
              <div className="run-header">
                <div>
                  <p className="eyebrow">Current run</p>
                  <h2>{runTitle}</h2>
                  <p>{activeRunId || "No active run yet"}</p>
                </div>
                <span
                  className={`tag ${runStatus === "failed" ? "tag-danger" : runStatus === "running" ? "tag-attention" : ""}`}
                >
                  {runStatus}
                </span>
              </div>
              <div className="stage-track" aria-label="Run stages">
                {runStages.map((stage) => (
                  <div
                    className={`stage stage-${stage.state}`}
                    key={stage.label}
                  >
                    <span />
                    <strong>{stage.label}</strong>
                  </div>
                ))}
              </div>
              <div className="task-ledger" aria-label="Task ledger">
                <div className="mini-heading">
                  <strong>Task ledger</strong>
                  <span>{workspaceTasks.length}</span>
                </div>
                {workspaceTasks.length > 0 ? (
                  workspaceTasks.map((task) => (
                    <div className="ledger-row" key={task.task_id}>
                      <div>
                        <strong>{taskRoleLabel(task)}</strong>
                        <span>{task.metadata?.purpose || task.runtime_id || "gateway"}</span>
                      </div>
                      <span className={`pill ${taskTone(task.status)}`}>
                        {task.status}
                      </span>
                    </div>
                  ))
                ) : (
                  <p className="empty compact-empty">
                    Task records appear when a session starts.
                  </p>
                )}
                {sessionDetail?.sandbox ? (
                  <div className="workspace-roots">
                    <span>Workspace</span>
                    <code>{sessionDetail.sandbox.workspace_root || "-"}</code>
                    <span>Artifacts</span>
                    <code>{sessionDetail.sandbox.artifact_root || "-"}</code>
                  </div>
                ) : null}
                {activeSessionId && sessionDetail?.session.status === "running" ? (
                  <button
                    className="button button-danger"
                    type="button"
                    onClick={() => void cancelSession(activeSessionId)}
                  >
                    Cancel session
                  </button>
                ) : null}
              </div>
              <div className="workspace-output">
                <span>Latest output</span>
                <p>
                  {latestOutput || "Output appears here once the run starts."}
                </p>
              </div>
              <div className="workspace-lists">
                <div>
                  <div className="mini-heading">
                    <strong>Live trace</strong>
                    <span>{traceEvents.length}</span>
                  </div>
                  <div className="event-list">
                    {traceEvents.slice(-5).map((event) => (
                      <button
                        className="event-row"
                        type="button"
                        key={event.event_id}
                        onClick={() =>
                          setExpandedEvents((current) => ({
                            ...current,
                            [event.event_id]: !current[event.event_id],
                          }))
                        }
                      >
                        <span>{event.type}</span>
                        <strong>
                          {eventOutput(event) ||
                            event.node_id ||
                            event.runtime_id ||
                            formatTime(event.time)}
                        </strong>
                      </button>
                    ))}
                    {traceEvents.length === 0 ? (
                      <p className="empty compact-empty">No trace events</p>
                    ) : null}
                  </div>
                </div>
                <div>
                  <div className="mini-heading">
                    <strong>Approvals</strong>
                    <span>{overview.pending_approvals.length}</span>
                  </div>
                  <div className="approval-queue">
                    {overview.pending_approvals.slice(0, 2).map((approval) => (
                      <div className="approval-card" key={approval.approval_id}>
                        <strong>{approval.summary}</strong>
                        <span>{approval.risk}</span>
                        <div>
                          <button
                            className="button button-secondary"
                            type="button"
                            disabled={mutatingApproval !== ""}
                            onClick={() =>
                              void resolveApproval(
                                approval.approval_id,
                                "grant",
                                "once",
                              )
                            }
                          >
                            Grant once
                          </button>
                          <button
                            className="button button-danger"
                            type="button"
                            disabled={mutatingApproval !== ""}
                            onClick={() =>
                              void resolveApproval(approval.approval_id, "deny")
                            }
                          >
                            Deny
                          </button>
                        </div>
                      </div>
                    ))}
                    {overview.pending_approvals.length === 0 ? (
                      <p className="empty compact-empty">
                        No pending approvals
                      </p>
                    ) : null}
                  </div>
                </div>
              </div>
            </section>
          </section>

          <details className="diagnostics">
            <summary>
              <span>Operational details</span>{" "}
              <strong>
                {overview.counts.models} models, {overview.counts.agents}{" "}
                agents, {blockedAgents.length} blocked entries
              </strong>
            </summary>
            <section className="workspace diagnostics-workspace">
              <section className="panel graph-panel" aria-label="Agent graph">
                <div className="panel-heading">
                  <div>
                    <h2>Graph</h2>
                    <p>
                      {overview.graph_snapshot?.project_id ??
                        "No graph snapshot yet. Run graph validate or install a pack."}
                    </p>
                  </div>
                  <span className="tag">
                    {overview.graph_snapshot ? "read-only" : "empty"}
                  </span>
                </div>
                <div className="canvas">
                  <ReactFlow
                    nodes={flow.nodes}
                    edges={flow.edges}
                    fitView
                    nodesDraggable={false}
                    nodesConnectable={false}
                    elementsSelectable={false}
                  >
                    <Background />
                    <Controls />
                  </ReactFlow>
                </div>
              </section>

              <section className="panel run-panel" aria-label="Run agent">
                <div className="panel-heading">
                  <div>
                    <h2>Run</h2>
                    <p>{activeRunId || "Start a supported graph agent"}</p>
                  </div>
                  <span
                    className={`tag ${runStatus === "failed" ? "tag-danger" : runStatus === "running" ? "tag-attention" : ""}`}
                  >
                    {runStatus}
                  </span>
                </div>
                <form className="run-form" onSubmit={startRun}>
                  <label>
                    <span>Agent</span>
                    <select
                      value={runAgentId}
                      onChange={(event) => setRunAgentId(event.target.value)}
                    >
                      {agentOptions.map((agent) => (
                        <option
                          value={agent.id}
                          key={agent.id}
                          disabled={!agent.supported}
                        >
                          {agent.id}
                          {agent.supported ? "" : ` - ${agent.reason}`}
                        </option>
                      ))}
                    </select>
                  </label>
                  {selectedAgent && !selectedAgent.supported ? (
                    <p className="form-hint">{selectedAgent.reason}</p>
                  ) : null}
                  <label>
                    <span>Prompt</span>
                    <textarea
                      rows={5}
                      value={runPrompt}
                      onChange={(event) => setRunPrompt(event.target.value)}
                      placeholder="Ask this agent to do one task"
                    />
                  </label>
                  {runError ? (
                    <div className="inline-error">{runError}</div>
                  ) : null}
                  <div className="run-actions">
                    <button
                      className="button"
                      type="submit"
                      disabled={
                        runStatus === "starting" ||
                        runStatus === "running" ||
                        !selectedAgent?.supported ||
                        runPrompt.trim() === ""
                      }
                    >
                      {runStatus === "starting" ? "Starting" : "Run"}
                    </button>
                    <span>{humanOutput(traceEvents) || "No output yet"}</span>
                  </div>
                </form>
              </section>

              <section className="panel" aria-label="Provider profiles">
                <div className="panel-heading">
                  <h2>Models</h2>
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
                  {overview.models.length === 0 ? (
                    <EmptyRow text="No models configured. Run model setup in this workspace." />
                  ) : null}
                </div>
              </section>

              <section className="panel" aria-label="Packs">
                <div className="panel-heading">
                  <h2>Packs</h2>
                  <span className="tag">{overview.packs.length}</span>
                </div>
                <div className="stack">
                  {overview.packs.map((pack) => (
                    <div className="list-item" key={pack.manifest.id}>
                      <div>
                        <strong>{pack.manifest.name}</strong>
                        <span>{packUsageText(pack)}</span>
                      </div>
                      <div className="list-meta">
                        <span
                          className={
                            pack.installed ? "pill pill-green" : "pill"
                          }
                        >
                          {pack.installed ? "installed" : "available"}
                        </span>
                        <span>{pack.manifest.trust?.level ?? "local"}</span>
                      </div>
                    </div>
                  ))}
                </div>
              </section>

              <section className="panel" aria-label="Runtimes">
                <div className="panel-heading">
                  <h2>Runtimes</h2>
                  <span className="tag">{overview.runtimes.length}</span>
                </div>
                <div className="table">
                  <div className="table-row table-head">
                    <span>Name</span>
                    <span>Kind</span>
                    <span>Status</span>
                    <span>Agents</span>
                  </div>
                  {overview.runtimes.map((runtime) => (
                    <div className="table-row" key={runtime.id}>
                      <span>{runtime.id}</span>
                      <span>{runtime.kind}</span>
                      <span>{runtime.status}</span>
                      <span>{runtime.agents?.join(", ") || "-"}</span>
                    </div>
                  ))}
                  {overview.runtimes.length === 0 ? (
                    <EmptyRow text="No runtimes in graph" />
                  ) : null}
                </div>
              </section>

              <section className="panel" aria-label="Recent runs">
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
                        setActiveSessionId(session.session_id);
                        setActiveRunId(session.run_id);
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
                  {overview.recent_sessions.length === 0 ? (
                    <p className="empty">No sessions recorded</p>
                  ) : null}
                </div>
              </section>

              <section className="panel" aria-label="Latest trace">
                <div className="panel-heading">
                  <h2>Trace</h2>
                  <span className="tag">{traceEvents.length}</span>
                </div>
                <div className="stack">
                  {traceEvents.map((event) => (
                    <div className="trace-item" key={event.event_id}>
                      <button
                        className="trace-summary"
                        type="button"
                        onClick={() =>
                          setExpandedEvents((current) => ({
                            ...current,
                            [event.event_id]: !current[event.event_id],
                          }))
                        }
                      >
                        <div>
                          <strong>
                            {event.sequence}. {event.type}
                          </strong>
                          <span>
                            {eventOutput(event) ||
                              event.node_id ||
                              event.runtime_id ||
                              event.run_id}
                          </span>
                        </div>
                        <div className="list-meta">
                          <span>{formatTime(event.time)}</span>
                          <span>{event.event_id}</span>
                        </div>
                      </button>
                      {expandedEvents[event.event_id] ? (
                        <pre className="payload">{formatPayload(event)}</pre>
                      ) : null}
                    </div>
                  ))}
                  {traceEvents.length === 0 ? (
                    <p className="empty">No trace events</p>
                  ) : null}
                </div>
              </section>

              <section className="panel" aria-label="Pending approvals">
                <div className="panel-heading">
                  <h2>Approvals</h2>
                  <span
                    className={
                      overview.pending_approvals.length > 0
                        ? "tag tag-attention"
                        : "tag"
                    }
                  >
                    {overview.pending_approvals.length}
                  </span>
                </div>
                {approvalError ? (
                  <div className="inline-error panel-inline">
                    {approvalError}
                  </div>
                ) : null}
                <div className="stack">
                  {overview.pending_approvals.map((approval) => (
                    <div className="approval-item" key={approval.approval_id}>
                      <div className="approval-copy">
                        <strong>{approval.summary}</strong>
                        <span>{approval.approval_id}</span>
                        <span>
                          {approval.requested_by_agent || approval.status}
                        </span>
                      </div>
                      <div className="approval-actions">
                        <span className="pill pill-amber">{approval.risk}</span>
                        <button
                          className="button button-secondary"
                          type="button"
                          disabled={mutatingApproval !== ""}
                          onClick={() =>
                            void resolveApproval(
                              approval.approval_id,
                              "grant",
                              "once",
                            )
                          }
                        >
                          Grant once
                        </button>
                        <button
                          className="button button-secondary"
                          type="button"
                          disabled={mutatingApproval !== ""}
                          onClick={() =>
                            void resolveApproval(
                              approval.approval_id,
                              "grant",
                              "run",
                            )
                          }
                        >
                          Grant run
                        </button>
                        <button
                          className="button button-danger"
                          type="button"
                          disabled={mutatingApproval !== ""}
                          onClick={() =>
                            void resolveApproval(approval.approval_id, "deny")
                          }
                        >
                          Deny
                        </button>
                      </div>
                    </div>
                  ))}
                  {overview.pending_approvals.length === 0 ? (
                    <p className="empty">No pending approvals</p>
                  ) : null}
                </div>
              </section>

              <section className="panel" aria-label="Unavailable actions">
                <div className="panel-heading">
                  <h2>Unavailable</h2>
                  <span className="tag">Gate 8</span>
                </div>
                <div className="stack">
                  {overview.unavailable.map((item) => (
                    <div className="list-item" key={item.name}>
                      <div>
                        <strong>{item.name}</strong>
                        <span>{item.reason}</span>
                      </div>
                      <span className="pill">{item.status}</span>
                    </div>
                  ))}
                </div>
              </section>
            </section>
          </details>
        </>
      ) : null}
    </main>
  );
}

function readTheme(): Theme {
  const saved = window.localStorage.getItem("nomici.console.theme");
  if (saved === "light" || saved === "dark") {
    return saved;
  }
  return window.matchMedia("(prefers-color-scheme: light)").matches
    ? "light"
    : "dark";
}

function Metric({
  label,
  value,
  tone,
}: {
  label: string;
  value: number;
  tone?: "attention";
}) {
  return (
    <div className={`metric ${tone === "attention" ? "metric-attention" : ""}`}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
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
        return {
          id,
          supported: Boolean(agent.model),
          reason: agent.model ? "" : "missing model",
        };
      }
      if (agent.kind !== "external_agent") {
        return {
          id,
          supported: false,
          reason: `${agent.kind} is not executable`,
        };
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
      if (chainCheck.supported) {
        return { id, supported: true, reason: "" };
      }
      return {
        id,
        supported: false,
        reason: chainCheck.reason,
      };
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
      return {
        supported: false,
        reason: "handoff chain has multiple outgoing edges",
      };
    }
    const edge = outgoing[0];
    if (edge.mode !== "handoff") {
      return {
        supported: false,
        reason: "only handoff chains are executable",
      };
    }
    const target = snapshot.ir.agents[edge.to];
    const targetRuntime = target?.runtime
      ? snapshot.ir.runtimes?.[target.runtime]
      : undefined;
    if (
      target?.kind !== "external_agent" ||
      targetRuntime?.kind !== "cli_agent"
    ) {
      return {
        supported: false,
        reason: "handoff target is not a cli_agent external agent",
      };
    }
    if (visited.has(edge.to)) {
      return {
        supported: false,
        reason: "handoff chain contains a cycle",
      };
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
    packs: next.packs ?? [],
    runtimes: next.runtimes ?? [],
    recent_runs: next.recent_runs ?? [],
    recent_sessions: next.recent_sessions ?? [],
    latest_trace: next.latest_trace ?? [],
    pending_approvals: next.pending_approvals ?? [],
    unavailable: next.unavailable ?? [],
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
  for (const key of [
    "output_preview",
    "stdout_preview",
    "stderr_preview",
    "message",
    "error",
  ]) {
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

type RunStage = {
  label: string;
  state: "waiting" | "active" | "done" | "blocked";
};

function buildRunStages(
  runStatus: "idle" | "starting" | "running" | "completed" | "failed",
  events: TraceEvent[],
): RunStage[] {
  const types = new Set(events.map((event) => event.type));
  const hasOutput = events.some((event) => eventOutput(event));
  const isTerminal = runStatus === "completed" || runStatus === "failed";
  const isBlocked =
    runStatus === "failed" ||
    types.has("approval.requested") ||
    types.has("policy.blocked");

  return [
    {
      label: "Intake",
      state: runStatus === "idle" ? "waiting" : "done",
    },
    {
      label: "Plan",
      state:
        events.length === 0
          ? runStatus === "starting"
            ? "active"
            : "waiting"
          : "done",
    },
    {
      label: "Execute",
      state:
        isTerminal || hasOutput
          ? "done"
          : runStatus === "running"
            ? "active"
            : "waiting",
    },
    {
      label: "Review",
      state: isBlocked ? "blocked" : isTerminal ? "done" : "waiting",
    },
    {
      label: "Deliver",
      state:
        runStatus === "completed"
          ? "done"
          : runStatus === "failed"
            ? "blocked"
            : "waiting",
    },
  ];
}

function formatPayload(event: TraceEvent): string {
  return JSON.stringify(
    {
      payload: event.payload ?? {},
      metadata: event.metadata ?? {},
      redactions: event.redactions ?? [],
    },
    null,
    2,
  );
}

function EmptyRow({ text }: { text: string }) {
  return (
    <div className="table-row empty-row">
      <span>{text}</span>
      <span>-</span>
      <span>-</span>
      <span>-</span>
    </div>
  );
}

function packUsageText(pack: PackStatus): string {
  if (pack.installed) {
    const entrypoint =
      pack.installation?.entrypoints?.[0] ??
      pack.manifest.agents?.entrypoints?.[0];
    return entrypoint
      ? `Run: nomici run ${entrypoint} "..."`
      : `${pack.manifest.id} installed`;
  }
  return `Install: nomici pack install ${pack.manifest.id} --model <profile>`;
}

function buildFlow(snapshot?: GraphSnapshot): { nodes: Node[]; edges: Edge[] } {
  if (!snapshot) {
    return {
      nodes: [
        {
          id: "empty",
          position: { x: 80, y: 80 },
          data: { label: "No graph snapshot" },
          type: "input",
        },
      ],
      edges: [],
    };
  }

  const modelIds = Object.keys(snapshot.ir.models).sort();
  const agentIds = Object.keys(snapshot.ir.agents).sort();
  const runtimeIds = Object.keys(snapshot.ir.runtimes ?? {}).sort();

  const nodes: Node[] = [
    ...modelIds.map((id, index) => ({
      id: `model:${id}`,
      position: { x: 40, y: 60 + index * 92 },
      data: { label: `model ${id}` },
      type: "input",
    })),
    ...agentIds.map((id, index) => ({
      id: `agent:${id}`,
      position: { x: 300, y: 40 + index * 96 },
      data: { label: `${snapshot.ir.agents[id].kind} ${id}` },
    })),
    ...runtimeIds.map((id, index) => ({
      id: `runtime:${id}`,
      position: { x: 650, y: 60 + index * 92 },
      data: { label: `runtime ${id}` },
      type: "output",
    })),
  ];

  const edges: Edge[] = [
    ...agentIds
      .filter((id) => snapshot.ir.agents[id].model)
      .map((id) => ({
        id: `model-edge:${id}`,
        source: `model:${snapshot.ir.agents[id].model}`,
        target: `agent:${id}`,
        label: "brain",
      })),
    ...agentIds
      .filter((id) => snapshot.ir.agents[id].runtime)
      .map((id) => ({
        id: `runtime-edge:${id}`,
        source: `agent:${id}`,
        target: `runtime:${snapshot.ir.agents[id].runtime}`,
        label: "runtime",
      })),
    ...snapshot.ir.edges.map((edge) => ({
      id: edge.id,
      source: `agent:${edge.from}`,
      target: `agent:${edge.to}`,
      label: edge.mode,
    })),
  ];

  return { nodes, edges };
}

function formatTime(value: string) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
