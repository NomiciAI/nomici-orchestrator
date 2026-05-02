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

type TraceEvent = {
  event_id: string;
  run_id: string;
  sequence: number;
  type: string;
  time: string;
  node_id?: string;
  runtime_id?: string;
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
        setError("Gateway token required");
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
            <h1>Agent Control Plane</h1>
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

      <section className="metric-strip" aria-label="Control plane metrics">
        <Metric label="Models" value={overview.counts.models} />
        <Metric label="Packs" value={overview.counts.packs_installed} />
        <Metric label="Agents" value={overview.counts.agents} />
        <Metric label="Runtimes" value={overview.counts.runtimes} />
        <Metric label="Runs" value={overview.counts.runs} />
        <Metric
          label="Approvals"
          value={overview.counts.pending_approvals}
          tone="attention"
        />
      </section>

      {status === "failed" ? (
        <div className="banner banner-error">{error}</div>
      ) : null}
      {status === "auth" ? (
        <form className="auth-banner" onSubmit={submitToken}>
          <div>
            <strong>Gateway token required</strong>
            <span>
              Run `nomici gateway token show` locally and paste the token here.
            </span>
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

      <section className="workspace">
        <section className="panel graph-panel" aria-label="Agent graph">
          <div className="panel-heading">
            <div>
              <h2>Graph</h2>
              <p>{overview.graph_snapshot?.project_id ?? "No snapshot"}</p>
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
              <EmptyRow text="No models configured" />
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
                  <span>{pack.manifest.id}</span>
                </div>
                <div className="list-meta">
                  <span className={pack.installed ? "pill pill-green" : "pill"}>
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
            <h2>Runs</h2>
            <span className="tag">{overview.recent_runs.length}</span>
          </div>
          <div className="stack">
            {overview.recent_runs.map((run) => (
              <div className="list-item" key={run.run_id}>
                <div>
                  <strong>{run.run_id}</strong>
                  <span>{run.last_type}</span>
                </div>
                <div className="list-meta">
                  <span>{run.event_count} events</span>
                  <span>{formatTime(run.last_time)}</span>
                </div>
              </div>
            ))}
            {overview.recent_runs.length === 0 ? (
              <p className="empty">No runs traced</p>
            ) : null}
          </div>
        </section>

        <section className="panel" aria-label="Latest trace">
          <div className="panel-heading">
            <h2>Trace</h2>
            <span className="tag">{overview.latest_trace.length}</span>
          </div>
          <div className="stack">
            {overview.latest_trace.map((event) => (
              <div className="list-item" key={event.event_id}>
                <div>
                  <strong>
                    {event.sequence}. {event.type}
                  </strong>
                  <span>
                    {event.node_id || event.runtime_id || event.run_id}
                  </span>
                </div>
                <div className="list-meta">
                  <span>{formatTime(event.time)}</span>
                  <span>{event.event_id}</span>
                </div>
              </div>
            ))}
            {overview.latest_trace.length === 0 ? (
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
          <div className="stack">
            {overview.pending_approvals.map((approval) => (
              <div className="list-item" key={approval.approval_id}>
                <div>
                  <strong>{approval.summary}</strong>
                  <span>{approval.approval_id}</span>
                </div>
                <div className="list-meta">
                  <span className="pill pill-amber">{approval.risk}</span>
                  <span>{approval.requested_by_agent || approval.status}</span>
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
    latest_trace: next.latest_trace ?? [],
    pending_approvals: next.pending_approvals ?? [],
    unavailable: next.unavailable ?? [],
  };
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
