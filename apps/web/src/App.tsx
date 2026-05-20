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
      {
        id: string;
        name?: string;
        description?: string;
        kind: string;
        model?: string;
        runtime?: string;
        role?: string;
        instructions?: string;
        tools?: string[];
        skills?: string[];
        tags?: string[];
      }
    >;
    edges: Array<{ id: string; from: string; to: string; mode: string }>;
  };
};

type RouteDecision = {
  mode: "workspace_run" | "clarify" | "direct_reply";
  goal: string;
  complexity: "simple" | "medium" | "long_horizon";
  recommended_agent_id?: string;
  selected_roles?: string[];
  needs_plan_review?: boolean;
  required_tools?: string[];
  required_skills?: string[];
  missing_inputs?: string[];
  risk?: string;
  confidence?: number;
  rationale?: string;
  clarification?: string;
  manual_agent_id?: string;
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

type ToolDefinition = {
  id: string;
  description: string;
  auth: string;
  network_risk: string;
  filesystem_risk: string;
  mutation_risk: string;
  allowed_scopes: string[];
  redaction_rules: string[];
  execution: string;
};

type SkillDefinition = {
  id: string;
  name: string;
  description: string;
  triggers?: string[];
  files?: string[];
  required_tools?: string[];
  risk?: string;
  compatibility?: string;
  briefing?: string;
  source?: string;
};

type RunSession = {
  session_id: string;
  run_id: string;
  project_id: string;
  graph_snapshot_id: string;
  title: string;
  source_channel: string;
  status: string;
  execution_state?: string;
  started_at: string;
  updated_at: string;
  completed_at?: string;
  metadata?: {
    route_decision?: RouteDecision;
    recommended_agent_id?: string;
    needs_plan_review?: boolean;
    chat_id?: string;
    message_id?: string;
  };
};

type RunTask = {
  task_id: string;
  run_id: string;
  agent_id: string;
  runtime_id?: string;
  status: string;
  context_snapshot_id?: string;
  blocked_reason?: string;
  selected_context_snapshot_id?: string;
  artifact_refs?: string[];
  started_at: string;
  updated_at: string;
  completed_at?: string;
  metadata?: {
    role_id?: string;
    purpose?: string;
    summary?: string;
    output_preview?: string;
    failure_reason?: string;
    selection_reason?: string;
    match_score?: number;
    skipped_roles?: Array<{ role_id: string; reason: string }>;
    required_tools?: string[];
    match_required_tools?: string[];
    output_contract?: {
      kind?: string;
      description?: string;
      required?: string[];
    };
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

type ArtifactContent = {
  artifact_id: string;
  path: string;
  content: string;
  truncated: boolean;
};

type ArtifactRevision = {
  revision_id: string;
  artifact_id: string;
  revision: number;
  review_state: string;
  diff_preview?: string;
  created_at: string;
};

type ToolCallRecord = {
  tool_call_id: string;
  task_id?: string;
  tool_id: string;
  status: string;
  risk?: string;
  input_preview?: string;
  output_preview?: string;
  artifact_refs?: string[];
  approval_id?: string;
  error?: string;
  updated_at: string;
};

type BlockedAction = {
  blocked_action_id: string;
  session_id: string;
  run_id: string;
  task_id?: string;
  kind: string;
  status: string;
  title: string;
  body?: string;
  required_action?: string;
  resume_target_task_id?: string;
  approval_id?: string;
  artifact_id?: string;
  tool_call_id?: string;
  metadata?: Record<string, unknown>;
  updated_at: string;
};

type RunSessionDetail = {
  session: RunSession;
  tasks: RunTask[];
  sandbox?: SandboxRecord;
  uploads?: UploadRecord[];
  artifacts?: ArtifactRecord[];
  tool_calls?: ToolCallRecord[];
  blocked_actions?: BlockedAction[];
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

type MemoryProposal = {
  proposal_id: string;
  title: string;
  body: string;
  status: string;
  context_id?: string;
  updated_at: string;
};

type MemoryItem = {
  context_id: string;
  title: string;
  body: string;
  tags?: string[];
  artifact_refs?: string[];
  updated_at: string;
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
  metadata?: {
    route_decision?: RouteDecision;
  };
};

type ChatDetail = {
  thread: ChatThread;
  messages: ChatMessage[];
};

type ChatMessageResponse = {
  message: ChatMessage;
  assistant_message?: ChatMessage;
  route_decision?: RouteDecision;
  clarification?: string;
  run?: {
    run_id: string;
    status: string;
    agent_id: string;
    session_id?: string;
  };
};

type AgentRecord = {
  id: string;
  name?: string;
  description?: string;
  kind: string;
  model?: string;
  runtime?: string;
  role?: string;
  instructions?: string;
  tools?: string[];
  skills?: string[];
  tags?: string[];
  triggers?: string[];
  approval_policy?: string;
  permissions?: Record<string, unknown>;
  runtime_profile?: Record<string, unknown>;
};

type OrchestrationConfig = {
  entrypoint?: string;
  role_order?: string[];
  disabled_roles?: string[];
  plan_review_policy?: string;
  roles?: Record<
    string,
    {
      purpose?: string;
      instructions?: string;
      output_contract?: {
        kind?: string;
        description?: string;
        required?: string[];
      };
      required_tools?: string[];
      required_skills?: string[];
      plan_review_policy?: string;
    }
  >;
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
  const [toolCatalog, setToolCatalog] = useState<ToolDefinition[]>([]);
  const [skillCatalog, setSkillCatalog] = useState<SkillDefinition[]>([]);
  const [agents, setAgents] = useState<AgentRecord[]>([]);
  const [orchestration, setOrchestration] = useState<OrchestrationConfig>({});
  const [chats, setChats] = useState<ChatThread[]>([]);
  const [chatDetail, setChatDetail] = useState<ChatDetail | null>(null);
  const [warnings, setWarnings] = useState<string[]>([]);
  const [status, setStatus] = useState<"loading" | "ready" | "failed" | "auth">(
    "loading",
  );
  const [error, setError] = useState("");
  const [runAgentId, setRunAgentId] = useState("auto");
  const [messageText, setMessageText] = useState("");
  const [selectedSkillIds, setSelectedSkillIds] = useState<string[]>([]);
  const [activeRouteDecision, setActiveRouteDecision] =
    useState<RouteDecision | null>(null);
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
  const [clarificationAnswer, setClarificationAnswer] = useState("");
  const [mutatingApproval, setMutatingApproval] = useState("");
  const [memoryProposals, setMemoryProposals] = useState<MemoryProposal[]>([]);
  const [memoryItems, setMemoryItems] = useState<MemoryItem[]>([]);
  const [reviewQueue, setReviewQueue] = useState<BlockedAction[]>([]);
  const [mutatingMemory, setMutatingMemory] = useState("");
  const [artifactContent, setArtifactContent] =
    useState<ArtifactContent | null>(null);
  const [artifactRevisions, setArtifactRevisions] = useState<
    ArtifactRevision[]
  >([]);
  const [artifactMutation, setArtifactMutation] = useState("");
  const [agentDraft, setAgentDraft] = useState<AgentRecord>({
    id: "",
    kind: "model_agent",
    model: "",
    role: "",
    instructions: "",
    tools: [],
    skills: [],
    tags: [],
    triggers: [],
  });
  const [settingsMutation, setSettingsMutation] = useState("");
  const [agentValidation, setAgentValidation] = useState("");

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
  const workspaceToolCalls = sessionDetail?.tool_calls ?? [];
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
    if (runAgentId !== "auto" && agentOptions.length > 0) {
      const current = agentOptions.find((agent) => agent.id === runAgentId);
      if (!current) {
        setRunAgentId("auto");
      }
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
    const timer = window.setInterval(
      () => {
        void pollRunEvents(activeRunId, state);
      },
      activeSessionId ? 2500 : 1200,
    );
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
      const [
        nextChats,
        catalog,
        nextTools,
        nextSkills,
        nextAgents,
        nextOrchestration,
        nextMemory,
        nextMemoryItems,
        nextReviewQueue,
      ] = await Promise.all([
          apiRequest<ChatThread[]>("/api/chats?limit=50", {}, nextToken),
          apiRequest<ProviderDefinition[]>(
            "/api/provider-catalog",
            {},
            nextToken,
          ),
          apiRequest<ToolDefinition[]>("/api/tools", {}, nextToken).catch(
            () => [],
          ),
          apiRequest<SkillDefinition[]>("/api/skills", {}, nextToken).catch(
            () => [],
          ),
          apiRequest<AgentRecord[]>("/api/agents", {}, nextToken).catch(
            () => [],
          ),
          apiRequest<OrchestrationConfig>(
            "/api/orchestration",
            {},
            nextToken,
          ).catch(() => ({})),
          apiRequest<MemoryProposal[]>(
            "/api/memory/proposals?status=proposed",
            {},
            nextToken,
          ).catch(() => []),
          apiRequest<MemoryItem[]>("/api/memory/items", {}, nextToken).catch(
            () => [],
          ),
          apiRequest<BlockedAction[]>(
            "/api/review-queue?status=open",
            {},
            nextToken,
          ).catch(() => []),
        ]);
      setChats(nextChats ?? []);
      setProviderCatalog(catalog ?? []);
      setToolCatalog(nextTools ?? []);
      setSkillCatalog(nextSkills ?? []);
      setAgents(nextAgents ?? []);
      setOrchestration(nextOrchestration ?? {});
      setMemoryProposals(nextMemory ?? []);
      setMemoryItems(nextMemoryItems ?? []);
      setReviewQueue(nextReviewQueue ?? []);
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
    const lastRun = [...detail.messages]
      .reverse()
      .find((message) => message.run_id);
    if (lastRun?.run_id) {
      setActiveRunId(lastRun.run_id);
      setActiveSessionId(lastRun.session_id ?? "");
      setRunStatus("running");
      setRunEvents([]);
      if (lastRun.session_id) {
        await loadSessionDetail(lastRun.session_id);
      }
    } else {
      setActiveRouteDecision(latestRouteDecision(detail.messages));
    }
  }

  async function sendMessage(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (runAgentId !== "auto" && !selectedAgent?.supported) {
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
          agent_id: runAgentId === "auto" ? "" : selectedAgent?.id,
          selected_skills: selectedSkillIds,
          prompt: content,
          content,
        }),
      });
      setActiveRouteDecision(response.route_decision ?? null);
      setMessageText("");
      if (response.run) {
        setActiveRunId(response.run.run_id);
        setActiveSessionId(response.run.session_id ?? "");
        setRunStatus("running");
        if (response.run.session_id) {
          await loadSessionDetail(response.run.session_id);
        }
      } else {
        setRunStatus("idle");
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
    setActiveRouteDecision(latestRouteDecision(detail.messages));
  }

  async function loadSessionDetail(sessionId: string) {
    if (!sessionId) {
      return;
    }
    const detail = await apiRequest<RunSessionDetail>(
      `/api/sessions/${encodeURIComponent(sessionId)}`,
    );
    setSessionDetail(detail);
    setActiveRouteDecision(detail.session.metadata?.route_decision ?? null);
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
          body:
            action === "grant" ? JSON.stringify({ scope: "run" }) : undefined,
        },
      );
      if (action === "grant" && activeSessionId) {
        const detail = await apiRequest<RunSessionDetail>(
          `/api/sessions/${encodeURIComponent(activeSessionId)}/resume`,
          { method: "POST" },
        ).catch(() => null);
        if (detail) {
          setSessionDetail(detail);
          setRunStatus("running");
        }
      }
      await loadOverview();
    } finally {
      setMutatingApproval("");
    }
  }

  async function resolveMemory(
    proposalID: string,
    action: "approve" | "reject" | "delete",
  ) {
    setMutatingMemory(`${proposalID}:${action}`);
    try {
      await apiRequest<MemoryProposal>(
        `/api/memory/proposals/${encodeURIComponent(proposalID)}/${action}`,
        { method: "POST" },
      );
      await loadOverview();
    } finally {
      setMutatingMemory("");
    }
  }

  async function deleteMemoryItem(contextID: string) {
    setMutatingMemory(`delete-item:${contextID}`);
    try {
      await apiRequest<{ status: string }>(
        `/api/memory/items/${encodeURIComponent(contextID)}`,
        { method: "DELETE" },
      );
      await loadOverview();
    } finally {
      setMutatingMemory("");
    }
  }

  async function resolveBlockedAction(
    blockedActionID: string,
    decision: "retry" | "skip" | "stop",
  ) {
    if (!activeSessionId) {
      return;
    }
    setWorkspaceError("");
    setWorkspaceMutation(`blocked:${blockedActionID}:${decision}`);
    try {
      const detail = await apiRequest<RunSessionDetail>(
        `/api/sessions/${encodeURIComponent(activeSessionId)}/blocked-actions/${encodeURIComponent(blockedActionID)}/resolve`,
        {
          method: "POST",
          body: JSON.stringify({
            decision,
            note:
              decision === "retry"
                ? "User requested retry from blocked action."
                : "User resolved blocked action from Console.",
            resume: decision !== "stop",
          }),
        },
      );
      setSessionDetail(detail);
      if (decision !== "stop") {
        setRunStatus("running");
      }
    } catch (blockedError) {
      setWorkspaceError(
        blockedError instanceof Error
          ? blockedError.message
          : "Blocked action could not be resolved",
      );
    } finally {
      setWorkspaceMutation("");
    }
  }

  async function inspectArtifact(artifactID: string) {
    setArtifactMutation(`inspect:${artifactID}`);
    setWorkspaceError("");
    try {
      const content = await apiRequest<ArtifactContent>(
        `/api/artifacts/${encodeURIComponent(artifactID)}/content`,
      );
      const revisions = await apiRequest<ArtifactRevision[]>(
        `/api/artifacts/${encodeURIComponent(artifactID)}/revisions`,
      ).catch(() => []);
      setArtifactContent(content);
      setArtifactRevisions(revisions);
    } catch (artifactError) {
      setWorkspaceError(
        artifactError instanceof Error
          ? artifactError.message
          : "Artifact content could not be loaded",
      );
    } finally {
      setArtifactMutation("");
    }
  }

  async function downloadArtifact(artifact: ArtifactRecord) {
    setArtifactMutation(`download:${artifact.artifact_id}`);
    setWorkspaceError("");
    try {
      const headers: Record<string, string> = {};
      if (gatewayToken.trim() !== "") {
        headers.Authorization = `Bearer ${gatewayToken.trim()}`;
      }
      const response = await fetch(
        `/api/artifacts/${encodeURIComponent(artifact.artifact_id)}/download`,
        { headers },
      );
      if (!response.ok) {
        throw new Error(`Artifact download returned ${response.status}`);
      }
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = artifact.title || artifact.artifact_id;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
    } catch (artifactError) {
      setWorkspaceError(
        artifactError instanceof Error
          ? artifactError.message
          : "Artifact could not be downloaded",
      );
    } finally {
      setArtifactMutation("");
    }
  }

  async function submitClarification(blockedActionID: string) {
    if (!activeSessionId || clarificationAnswer.trim() === "") {
      return;
    }
    setWorkspaceError("");
    setWorkspaceMutation("clarification");
    try {
      const detail = await apiRequest<RunSessionDetail>(
        `/api/sessions/${encodeURIComponent(activeSessionId)}/clarifications`,
        {
          method: "POST",
          body: JSON.stringify({
            blocked_action_id: blockedActionID,
            answer: clarificationAnswer,
          }),
        },
      );
      setClarificationAnswer("");
      setSessionDetail(detail);
      setRunStatus("running");
    } catch (clarificationError) {
      setWorkspaceError(
        clarificationError instanceof Error
          ? clarificationError.message
          : "Clarification could not be submitted",
      );
    } finally {
      setWorkspaceMutation("");
    }
  }

  async function validateAgentDraft() {
    setAgentValidation("");
    setSettingsMutation("agent-validate");
    try {
      await apiRequest<{ status: string }>(
        `/api/agents/${encodeURIComponent(agentDraft.id || "draft")}/validate`,
        {
          method: "POST",
          body: JSON.stringify(agentDraft),
        },
      );
      setAgentValidation("valid");
    } catch (validationError) {
      setAgentValidation(
        validationError instanceof Error
          ? validationError.message
          : "Agent validation failed",
      );
    } finally {
      setSettingsMutation("");
    }
  }

  async function saveAgent(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (agentDraft.id.trim() === "") {
      setRunError("Agent id is required.");
      return;
    }
    setSettingsMutation("agent");
    setRunError("");
    setAgentValidation("");
    try {
      await apiRequest<AgentRecord>("/api/agents", {
        method: "POST",
        body: JSON.stringify({
          ...agentDraft,
          tools: splitCSV(agentDraft.tools?.join(",") ?? ""),
          skills: splitCSV(agentDraft.skills?.join(",") ?? ""),
          tags: splitCSV(agentDraft.tags?.join(",") ?? ""),
          triggers: splitCSV(agentDraft.triggers?.join(",") ?? ""),
        }),
      });
      setAgentDraft({
        id: "",
        kind: "model_agent",
        model: "",
        role: "",
        instructions: "",
        tools: [],
        skills: [],
        tags: [],
        triggers: [],
      });
      await loadOverview();
    } catch (saveError) {
      setRunError(
        saveError instanceof Error
          ? saveError.message
          : "Agent could not be saved",
      );
    } finally {
      setSettingsMutation("");
    }
  }

  async function saveOrchestration(next: OrchestrationConfig) {
    setSettingsMutation("orchestration");
    setRunError("");
    try {
      const saved = await apiRequest<OrchestrationConfig>(
        "/api/orchestration",
        {
          method: "PATCH",
          body: JSON.stringify(next),
        },
      );
      setOrchestration(saved);
      await loadOverview();
    } catch (saveError) {
      setRunError(
        saveError instanceof Error
          ? saveError.message
          : "Role flow could not be saved",
      );
    } finally {
      setSettingsMutation("");
    }
  }

  return (
    <main className={`shell theme-${theme}`}>
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
          {chats.length === 0 ? (
            <p className="empty compact-empty">No chats yet</p>
          ) : null}
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
        {status === "failed" ? (
          <div className="banner banner-error">{error}</div>
        ) : null}
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
                  <article
                    className={`message message-${message.role}`}
                    key={message.message_id}
                  >
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
                  <details className="advanced-agent-picker">
                    <summary>
                      Agent: {runAgentId === "auto" ? "Auto" : runAgentId}
                    </summary>
                    <select
                      value={runAgentId}
                      onChange={(event) => setRunAgentId(event.target.value)}
                    >
                      <option value="auto">Auto</option>
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
                  </details>
                  <details className="advanced-agent-picker">
                    <summary>Skills: {selectedSkillIds.length || "Auto"}</summary>
                    <div className="checkbox-grid composer-skill-grid">
                      {skillCatalog.map((skill) => (
                        <label key={skill.id}>
                          <input
                            type="checkbox"
                            checked={selectedSkillIds.includes(skill.id)}
                            onChange={() =>
                              setSelectedSkillIds(
                                toggleListValue(selectedSkillIds, skill.id),
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
                      runStatus === "starting" ||
                      runStatus === "running" ||
                      (runAgentId !== "auto" && !selectedAgent?.supported) ||
                      messageText.trim() === ""
                    }
                  >
                    {runStatus === "starting" ? "Starting" : "Send"}
                  </button>
                </div>
                {runError ? (
                  <div className="inline-error">{runError}</div>
                ) : null}
              </form>
            </section>
            <WorkspacePanel
              activeRunId={activeRunId}
              runStatus={runStatus}
              routeDecision={activeRouteDecision}
              sessionDetail={sessionDetail}
              traceEvents={traceEvents}
              tasks={workspaceTasks}
              uploads={workspaceUploads}
              artifacts={workspaceArtifacts}
              toolCalls={workspaceToolCalls}
              approvals={overview.pending_approvals}
              memoryProposals={memoryProposals}
              memoryItems={memoryItems}
              artifactContent={artifactContent}
              artifactRevisions={artifactRevisions}
              artifactMutation={artifactMutation}
              planArtifact={sessionNeedsPlanReview ? planArtifact : undefined}
              planRevision={planRevision}
              setPlanRevision={setPlanRevision}
              uploadFile={uploadFile}
              setUploadFile={setUploadFile}
              workspaceError={workspaceError}
              workspaceMutation={workspaceMutation}
              mutatingApproval={mutatingApproval}
              mutatingMemory={mutatingMemory}
              clarificationAnswer={clarificationAnswer}
              setClarificationAnswer={setClarificationAnswer}
              onApprovePlan={() => void approvePlan()}
              onRevisePlan={() => void revisePlan()}
              onUpload={() => void uploadInput()}
              onCancel={() => void cancelSession()}
              onResolveApproval={(approvalID, action) =>
                void resolveApproval(approvalID, action)
              }
              onResolveMemory={(proposalID, action) =>
                void resolveMemory(proposalID, action)
              }
              onDeleteMemoryItem={(contextID) => void deleteMemoryItem(contextID)}
              onSubmitClarification={(blockedActionID) =>
                void submitClarification(blockedActionID)
              }
              onResolveBlockedAction={(blockedActionID, decision) =>
                void resolveBlockedAction(blockedActionID, decision)
              }
              onInspectArtifact={(artifactID) => void inspectArtifact(artifactID)}
              onDownloadArtifact={(artifact) => void downloadArtifact(artifact)}
            />
          </section>
        ) : null}

        {isAuthenticated && view === "orchestrate" ? (
          <section className="workspace diagnostics-workspace">
            <WorkspacePanel
              activeRunId={activeRunId}
              runStatus={runStatus}
              routeDecision={activeRouteDecision}
              sessionDetail={sessionDetail}
              traceEvents={traceEvents}
              tasks={workspaceTasks}
              uploads={workspaceUploads}
              artifacts={workspaceArtifacts}
              toolCalls={workspaceToolCalls}
              approvals={overview.pending_approvals}
              memoryProposals={memoryProposals}
              memoryItems={memoryItems}
              artifactContent={artifactContent}
              artifactRevisions={artifactRevisions}
              artifactMutation={artifactMutation}
              planArtifact={sessionNeedsPlanReview ? planArtifact : undefined}
              planRevision={planRevision}
              setPlanRevision={setPlanRevision}
              uploadFile={uploadFile}
              setUploadFile={setUploadFile}
              workspaceError={workspaceError}
              workspaceMutation={workspaceMutation}
              mutatingApproval={mutatingApproval}
              mutatingMemory={mutatingMemory}
              clarificationAnswer={clarificationAnswer}
              setClarificationAnswer={setClarificationAnswer}
              onApprovePlan={() => void approvePlan()}
              onRevisePlan={() => void revisePlan()}
              onUpload={() => void uploadInput()}
              onCancel={() => void cancelSession()}
              onResolveApproval={(approvalID, action) =>
                void resolveApproval(approvalID, action)
              }
              onResolveMemory={(proposalID, action) =>
                void resolveMemory(proposalID, action)
              }
              onDeleteMemoryItem={(contextID) => void deleteMemoryItem(contextID)}
              onSubmitClarification={(blockedActionID) =>
                void submitClarification(blockedActionID)
              }
              onResolveBlockedAction={(blockedActionID, decision) =>
                void resolveBlockedAction(blockedActionID, decision)
              }
              onInspectArtifact={(artifactID) => void inspectArtifact(artifactID)}
              onDownloadArtifact={(artifact) => void downloadArtifact(artifact)}
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
            <section className="panel" aria-label="Review queue">
              <div className="panel-heading">
                <h2>Review queue</h2>
                <span className="tag">{reviewQueue.length}</span>
              </div>
              <div className="stack">
                {reviewQueue.slice(0, 8).map((action) => (
                  <button
                    className="list-item list-button"
                    key={action.blocked_action_id}
                    type="button"
                    onClick={() => {
                      setActiveRunId(action.run_id);
                      setActiveSessionId(action.session_id);
                      void loadSessionDetail(action.session_id);
                    }}
                  >
                    <div>
                      <strong>{action.title}</strong>
                      <span>{action.kind}</span>
                    </div>
                    <div className="list-meta">
                      <span>{action.status}</span>
                      <span>{formatTime(action.updated_at)}</span>
                    </div>
                  </button>
                ))}
                {reviewQueue.length === 0 ? (
                  <p className="empty compact-empty">No open review items.</p>
                ) : null}
              </div>
            </section>
            <OrchestrateBuilder
              agents={agents}
              orchestration={orchestration}
              toolCatalog={toolCatalog}
              skillCatalog={skillCatalog}
              saving={settingsMutation === "orchestration"}
              onSave={(next) => void saveOrchestration(next)}
            />
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
            <AgentBuilder
              agents={agents}
              models={overview.models}
              graphSnapshot={overview.graph_snapshot}
              toolCatalog={toolCatalog}
              skillCatalog={skillCatalog}
              draft={agentDraft}
              setDraft={setAgentDraft}
              saving={settingsMutation === "agent"}
              validating={settingsMutation === "agent-validate"}
              validation={agentValidation}
              onValidate={() => void validateAgentDraft()}
              onSave={(event) => void saveAgent(event)}
            />
          </section>
        ) : null}
      </section>
    </main>
  );
}

function WorkspacePanel({
  activeRunId,
  runStatus,
  routeDecision,
  sessionDetail,
  traceEvents,
  tasks,
  uploads,
  artifacts,
  toolCalls,
  approvals,
  memoryProposals,
  memoryItems,
  artifactContent,
  artifactRevisions,
  artifactMutation,
  planArtifact,
  planRevision,
  setPlanRevision,
  uploadFile,
  setUploadFile,
  workspaceError,
  workspaceMutation,
  mutatingApproval,
  mutatingMemory,
  clarificationAnswer,
  setClarificationAnswer,
  onApprovePlan,
  onRevisePlan,
  onUpload,
  onCancel,
  onResolveApproval,
  onResolveMemory,
  onDeleteMemoryItem,
  onSubmitClarification,
  onResolveBlockedAction,
  onInspectArtifact,
  onDownloadArtifact,
}: {
  activeRunId: string;
  runStatus: string;
  routeDecision: RouteDecision | null;
  sessionDetail: RunSessionDetail | null;
  traceEvents: TraceEvent[];
  tasks: RunTask[];
  uploads: UploadRecord[];
  artifacts: ArtifactRecord[];
  toolCalls: ToolCallRecord[];
  approvals: Approval[];
  memoryProposals: MemoryProposal[];
  memoryItems: MemoryItem[];
  artifactContent: ArtifactContent | null;
  artifactRevisions: ArtifactRevision[];
  artifactMutation: string;
  planArtifact?: ArtifactRecord;
  planRevision: string;
  setPlanRevision: (value: string) => void;
  uploadFile: File | null;
  setUploadFile: (value: File | null) => void;
  workspaceError: string;
  workspaceMutation: string;
  mutatingApproval: string;
  mutatingMemory: string;
  clarificationAnswer: string;
  setClarificationAnswer: (value: string) => void;
  onApprovePlan: () => void;
  onRevisePlan: () => void;
  onUpload: () => void;
  onCancel: () => void;
  onResolveApproval: (approvalID: string, action: "grant" | "deny") => void;
  onResolveMemory: (
    proposalID: string,
    action: "approve" | "reject" | "delete",
  ) => void;
  onDeleteMemoryItem: (contextID: string) => void;
  onSubmitClarification: (blockedActionID: string) => void;
  onResolveBlockedAction: (
    blockedActionID: string,
    decision: "retry" | "skip" | "stop",
  ) => void;
  onInspectArtifact: (artifactID: string) => void;
  onDownloadArtifact: (artifact: ArtifactRecord) => void;
}) {
  const latestOutput = humanOutput(traceEvents);
  const decision =
    routeDecision ?? sessionDetail?.session.metadata?.route_decision ?? null;
  const openBlockedActions = (sessionDetail?.blocked_actions ?? []).filter(
    (action) => action.status === "open",
  );
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

      {decision ? (
        <div className="route-decision">
          <div className="mini-heading no-border">
            <strong>Route decision</strong>
            <span>{decision.mode}</span>
          </div>
          <div className="route-grid">
            <span>Complexity</span>
            <strong>{decision.complexity}</strong>
            <span>Agent</span>
            <strong>{decision.recommended_agent_id || "auto"}</strong>
            <span>Plan review</span>
            <strong>{decision.needs_plan_review ? "required" : "auto"}</strong>
            <span>Risk</span>
            <strong>{decision.risk || "medium"}</strong>
            <span>Confidence</span>
            <strong>
              {decision.confidence
                ? `${Math.round(decision.confidence * 100)}%`
                : "heuristic"}
            </strong>
          </div>
          {decision.rationale ? <p>{decision.rationale}</p> : null}
          {decision.required_tools?.length ? (
            <div className="chip-row">
              {decision.required_tools.map((tool) => (
                <span className="chip" key={tool}>
                  {tool}
                </span>
              ))}
            </div>
          ) : null}
          {decision.required_skills?.length ? (
            <div className="chip-row">
              {decision.required_skills.map((skill) => (
                <span className="chip" key={skill}>
                  {skill}
                </span>
              ))}
            </div>
          ) : null}
          {decision.missing_inputs?.length ? (
            <p>Missing: {decision.missing_inputs.join(", ")}</p>
          ) : null}
        </div>
      ) : null}

      {openBlockedActions.length > 0 ? (
        <div className="blocked-panel">
          <div className="mini-heading no-border">
            <strong>Needs input</strong>
            <span>{openBlockedActions.length}</span>
          </div>
          {openBlockedActions.map((action) => (
            <div className="blocked-card" key={action.blocked_action_id}>
              <div>
                <strong>{action.title}</strong>
                <span>{action.body || action.required_action}</span>
                <small>
                  {action.kind}
                  {action.approval_id ? ` / ${action.approval_id}` : ""}
                  {action.tool_call_id ? ` / ${action.tool_call_id}` : ""}
                </small>
              </div>
              {action.kind === "clarification" ? (
                <div className="clarification-form">
                  <textarea
                    value={clarificationAnswer}
                    onChange={(event) =>
                      setClarificationAnswer(event.target.value)
                    }
                    placeholder="Answer the blocking question"
                  />
                  <button
                    className="button"
                    type="button"
                    disabled={
                      workspaceMutation !== "" ||
                      clarificationAnswer.trim() === ""
                    }
                    onClick={() =>
                      onSubmitClarification(action.blocked_action_id)
                    }
                  >
                    Submit
                  </button>
                </div>
              ) : action.kind === "retry_decision" ||
                action.kind === "tool_risk_review" ? (
                <div className="blocked-actions">
                  <button
                    className="button button-secondary"
                    type="button"
                    disabled={workspaceMutation !== ""}
                    onClick={() =>
                      onResolveBlockedAction(action.blocked_action_id, "retry")
                    }
                  >
                    Retry
                  </button>
                  <button
                    className="button button-secondary"
                    type="button"
                    disabled={workspaceMutation !== ""}
                    onClick={() =>
                      onResolveBlockedAction(action.blocked_action_id, "skip")
                    }
                  >
                    Skip
                  </button>
                  <button
                    className="button button-danger"
                    type="button"
                    disabled={workspaceMutation !== ""}
                    onClick={() =>
                      onResolveBlockedAction(action.blocked_action_id, "stop")
                    }
                  >
                    Stop
                  </button>
                </div>
              ) : null}
            </div>
          ))}
        </div>
      ) : null}

      {tasks.length > 0 ? <RoleTimeline tasks={tasks} /> : null}

      <div className="task-ledger">
        <div className="mini-heading">
          <strong>Task ledger</strong>
          <span>{tasks.length}</span>
        </div>
        {tasks.map((task) => (
          <div className="ledger-row" key={task.task_id}>
            <div>
              <strong>{taskRoleLabel(task)}</strong>
              <span>
                {task.metadata?.summary ||
                  task.metadata?.purpose ||
                  task.blocked_reason ||
                  "pending"}
              </span>
              <small>
                {task.started_at ? formatTime(task.started_at) : "-"} /{" "}
                {task.completed_at
                  ? formatTime(task.completed_at)
                  : task.updated_at
                    ? formatTime(task.updated_at)
                    : "-"}
                {task.artifact_refs?.length
                  ? ` / artifacts ${task.artifact_refs.length}`
                  : ""}
                {task.metadata?.selection_reason
                  ? ` / ${task.metadata.selection_reason}`
                  : ""}
              </small>
            </div>
            <span className={`pill ${taskTone(task.status)}`}>
              {task.status}
            </span>
          </div>
        ))}
        {tasks.length === 0 ? (
          <p className="empty compact-empty">
            Task records appear when a run starts.
          </p>
        ) : null}
        {sessionDetail?.sandbox ? (
          <div className="workspace-roots">
            <span>Workspace</span>
            <code>{sessionDetail.sandbox.workspace_root || "-"}</code>
            <span>Artifacts</span>
            <code>{sessionDetail.sandbox.artifact_root || "-"}</code>
            <span>Sandbox</span>
            <code>
              {sessionDetail.sandbox.provider} /{" "}
              {sessionDetail.sandbox.cleanup_status}
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
              onChange={(event) =>
                setUploadFile(event.target.files?.[0] ?? null)
              }
            />
            <button
              className="button button-secondary"
              type="button"
              disabled={
                !uploadFile || !sessionDetail || workspaceMutation !== ""
              }
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
            <div className="artifact-row" key={artifact.artifact_id}>
              <div>
                <span>
                  {artifact.type} / {artifact.review_state} / r
                  {artifact.revision}
                </span>
                <strong>{artifact.title}</strong>
                {artifact.task_id ? <small>{artifact.task_id}</small> : null}
              </div>
              <div>
                <button
                  className="button button-secondary"
                  type="button"
                  disabled={artifactMutation !== ""}
                  onClick={() => onInspectArtifact(artifact.artifact_id)}
                >
                  Preview
                </button>
                <button
                  className="button button-secondary"
                  type="button"
                  disabled={artifactMutation !== "" || !artifact.path}
                  onClick={() => onDownloadArtifact(artifact)}
                >
                  Download
                </button>
              </div>
            </div>
          ))}
          {artifactContent ? (
            <div className="artifact-detail">
              <div className="mini-heading no-border">
                <strong>{artifactContent.artifact_id}</strong>
                <span>{artifactContent.truncated ? "truncated" : "full"}</span>
              </div>
              <code>{artifactContent.path}</code>
              <pre>{artifactContent.content}</pre>
              {artifactRevisions.length > 0 ? (
                <div className="revision-list">
                  <div className="mini-heading no-border">
                    <strong>Revisions</strong>
                    <span>{artifactRevisions.length}</span>
                  </div>
                  {artifactRevisions.slice(0, 5).map((revision) => (
                    <div
                      className="event-row passive-row"
                      key={revision.revision_id}
                    >
                      <span>
                        r{revision.revision} / {revision.review_state}
                      </span>
                      <strong>{formatTime(revision.created_at)}</strong>
                      {revision.diff_preview ? (
                        <small>{revision.diff_preview}</small>
                      ) : null}
                    </div>
                  ))}
                </div>
              ) : null}
            </div>
          ) : null}
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
            <strong>Tool calls</strong>
            <span>{toolCalls.length}</span>
          </div>
          {toolCalls.slice(0, 5).map((call) => (
            <div className="event-row passive-row" key={call.tool_call_id}>
              <span>{call.tool_id}</span>
              <strong>
                {call.status}
                {call.approval_id ? ` / ${call.approval_id}` : ""}
              </strong>
              {call.output_preview || call.error ? (
                <small>{call.output_preview || call.error}</small>
              ) : null}
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
                  onClick={() =>
                    onResolveApproval(approval.approval_id, "grant")
                  }
                >
                  Grant
                </button>
                <button
                  className="button button-danger"
                  type="button"
                  disabled={mutatingApproval !== ""}
                  onClick={() =>
                    onResolveApproval(approval.approval_id, "deny")
                  }
                >
                  Deny
                </button>
              </div>
            </div>
          ))}
        </div>
        <div>
          <div className="mini-heading">
            <strong>Memory</strong>
            <span>{memoryProposals.length + memoryItems.length}</span>
          </div>
          {memoryProposals.slice(0, 3).map((proposal) => (
            <div className="approval-card" key={proposal.proposal_id}>
              <strong>{proposal.title}</strong>
              <span>{proposal.status}</span>
              <p>{proposal.body}</p>
              <div>
                <button
                  className="button button-secondary"
                  type="button"
                  disabled={mutatingMemory !== ""}
                  onClick={() =>
                    onResolveMemory(proposal.proposal_id, "approve")
                  }
                >
                  Approve
                </button>
                <button
                  className="button button-danger"
                  type="button"
                  disabled={mutatingMemory !== ""}
                  onClick={() =>
                    onResolveMemory(proposal.proposal_id, "reject")
                  }
                >
                  Reject
                </button>
              </div>
            </div>
          ))}
          {memoryItems.slice(0, 3).map((item) => (
            <div className="approval-card" key={item.context_id}>
              <strong>{item.title}</strong>
              <span>approved</span>
              <p>{item.body}</p>
              <div>
                <button
                  className="button button-danger"
                  type="button"
                  disabled={mutatingMemory !== ""}
                  onClick={() => onDeleteMemoryItem(item.context_id)}
                >
                  Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      </div>

      {workspaceError ? (
        <div className="inline-error panel-inline">{workspaceError}</div>
      ) : null}
      {sessionDetail?.session.status === "running" ? (
        <button
          className="button button-danger workspace-cancel"
          type="button"
          onClick={onCancel}
        >
          Cancel session
        </button>
      ) : null}
    </section>
  );
}

function RoleTimeline({ tasks }: { tasks: RunTask[] }) {
  return (
    <div className="role-flow" aria-label="Role flow">
      {tasks.map((task, index) => (
        <div
          className={`role-step ${taskTone(task.status)}`}
          key={task.task_id}
        >
          <span>{index + 1}</span>
          <strong>{taskRoleLabel(task)}</strong>
          <small>{task.status}</small>
        </div>
      ))}
    </div>
  );
}

function OrchestrateBuilder({
  agents,
  orchestration,
  toolCatalog,
  skillCatalog,
  saving,
  onSave,
}: {
  agents: AgentRecord[];
  orchestration: OrchestrationConfig;
  toolCatalog: ToolDefinition[];
  skillCatalog: SkillDefinition[];
  saving: boolean;
  onSave: (next: OrchestrationConfig) => void;
}) {
  const modelAgents = agents.filter((agent) => agent.kind !== "tool_agent");
  const [draft, setDraft] = useState<OrchestrationConfig>(orchestration);
  const [selectedRole, setSelectedRole] = useState("");
  const [draggingRole, setDraggingRole] = useState("");
  useEffect(() => {
    setDraft(orchestration);
  }, [orchestration]);
  const roleOrder = draft.role_order?.length
    ? draft.role_order
    : modelAgents.map((agent) => agent.id);
  const disabled = new Set(draft.disabled_roles ?? []);
  const currentRole = selectedRole || roleOrder[0] || "";
  const currentRoleConfig = draft.roles?.[currentRole] ?? {};
  const updateRole = (
    roleID: string,
    patch: NonNullable<OrchestrationConfig["roles"]>[string],
  ) =>
    setDraft({
      ...draft,
      roles: {
        ...(draft.roles ?? {}),
        [roleID]: { ...(draft.roles?.[roleID] ?? {}), ...patch },
      },
    });
  const moveRole = (roleID: string, direction: -1 | 1) => {
    const next = [...roleOrder];
    const index = next.indexOf(roleID);
    const target = index + direction;
    if (index < 0 || target < 0 || target >= next.length) {
      return;
    }
    [next[index], next[target]] = [next[target], next[index]];
    setDraft({ ...draft, role_order: next });
  };
  const dropRole = (targetRoleID: string) => {
    if (!draggingRole || draggingRole === targetRoleID) {
      setDraggingRole("");
      return;
    }
    const next = roleOrder.filter((roleID) => roleID !== draggingRole);
    const targetIndex = next.indexOf(targetRoleID);
    next.splice(targetIndex < 0 ? next.length : targetIndex, 0, draggingRole);
    setDraft({ ...draft, role_order: next });
    setDraggingRole("");
  };
  const normalizedDraft = { ...draft, role_order: roleOrder };
  const hasDiff =
    JSON.stringify(normalizedDraft) !== JSON.stringify(orchestration ?? {});
  return (
    <section className="panel" aria-label="Role flow builder">
      <div className="panel-heading">
        <div>
          <h2>Role flow</h2>
          <p>Sequential MVP flow</p>
        </div>
        <span className="tag">{roleOrder.length}</span>
      </div>
      <div className="builder-grid">
        <label>
          <span>Entrypoint</span>
          <select
            value={draft.entrypoint ?? ""}
            onChange={(event) =>
              setDraft({ ...draft, entrypoint: event.target.value })
            }
            disabled={saving}
          >
            <option value="">Auto</option>
            {agents.map((agent) => (
              <option value={agent.id} key={agent.id}>
                {agent.id}
              </option>
            ))}
          </select>
        </label>
        <label>
          <span>Plan review</span>
          <select
            value={draft.plan_review_policy ?? "auto"}
            onChange={(event) =>
              setDraft({
                ...draft,
                plan_review_policy: event.target.value,
              })
            }
            disabled={saving}
          >
            <option value="auto">Auto</option>
            <option value="always">Always</option>
            <option value="never">Never</option>
          </select>
        </label>
      </div>
      <div className="role-library">
        {roleOrder.map((roleID) => (
          <div
            className={`role-toggle ${disabled.has(roleID) ? "role-disabled" : ""} ${
              currentRole === roleID ? "role-selected" : ""
            }`}
            key={roleID}
            draggable
            onDragStart={() => setDraggingRole(roleID)}
            onDragOver={(event) => event.preventDefault()}
            onDrop={() => dropRole(roleID)}
          >
            <button
              type="button"
              disabled={saving}
              onClick={() => setSelectedRole(roleID)}
            >
              <strong>{roleID}</strong>
              <span>{disabled.has(roleID) ? "disabled" : "enabled"}</span>
            </button>
            <div>
              <button
                type="button"
                disabled={saving}
                onClick={() => moveRole(roleID, -1)}
              >
                Up
              </button>
              <button
                type="button"
                disabled={saving}
                onClick={() => moveRole(roleID, 1)}
              >
                Down
              </button>
              <button
                type="button"
                disabled={saving}
                onClick={() => {
                  const nextDisabled = new Set(disabled);
                  if (nextDisabled.has(roleID)) {
                    nextDisabled.delete(roleID);
                  } else {
                    nextDisabled.add(roleID);
                  }
                  setDraft({
                    ...draft,
                    role_order: roleOrder,
                    disabled_roles: [...nextDisabled],
                  });
                }}
              >
                {disabled.has(roleID) ? "Enable" : "Disable"}
              </button>
            </div>
          </div>
        ))}
      </div>
      {currentRole ? (
        <div className="builder-form role-config">
          <div className="mini-heading">
            <strong>{currentRole}</strong>
            <span>Role config</span>
          </div>
          <label>
            <span>Purpose</span>
            <input
              value={currentRoleConfig.purpose ?? ""}
              onChange={(event) =>
                updateRole(currentRole, { purpose: event.target.value })
              }
              placeholder="Role purpose"
            />
          </label>
          <label>
            <span>Instructions</span>
            <textarea
              rows={3}
              value={currentRoleConfig.instructions ?? ""}
              onChange={(event) =>
                updateRole(currentRole, { instructions: event.target.value })
              }
              placeholder="Role-specific operating instructions"
            />
          </label>
          <label>
            <span>Output contract</span>
            <input
              value={currentRoleConfig.output_contract?.description ?? ""}
              onChange={(event) =>
                updateRole(currentRole, {
                  output_contract: {
                    ...(currentRoleConfig.output_contract ?? {}),
                    description: event.target.value,
                  },
                })
              }
              placeholder="Expected deliverable"
            />
          </label>
          <label>
            <span>Role plan review</span>
            <select
              value={currentRoleConfig.plan_review_policy ?? "auto"}
              onChange={(event) =>
                updateRole(currentRole, {
                  plan_review_policy: event.target.value,
                })
              }
              disabled={saving}
            >
              <option value="auto">Auto</option>
              <option value="required">Required</option>
              <option value="disabled">Disabled</option>
            </select>
          </label>
          <div className="selection-panel">
            <div className="mini-heading no-border">
              <strong>Required tools</strong>
              <span>{currentRoleConfig.required_tools?.length ?? 0}</span>
            </div>
            <div className="checkbox-grid">
              {toolCatalog.map((tool) => (
                <label key={tool.id}>
                  <input
                    type="checkbox"
                    checked={
                      currentRoleConfig.required_tools?.includes(tool.id) ??
                      false
                    }
                    onChange={() =>
                      updateRole(currentRole, {
                        required_tools: toggleListValue(
                          currentRoleConfig.required_tools ?? [],
                          tool.id,
                        ),
                      })
                    }
                    disabled={saving}
                  />
                  <span>{tool.id}</span>
                  <small>{tool.mutation_risk}</small>
                </label>
              ))}
            </div>
          </div>
          <div className="selection-panel">
            <div className="mini-heading no-border">
              <strong>Required skills</strong>
              <span>{currentRoleConfig.required_skills?.length ?? 0}</span>
            </div>
            <div className="checkbox-grid">
              {skillCatalog.map((skill) => (
                <label key={skill.id}>
                  <input
                    type="checkbox"
                    checked={
                      currentRoleConfig.required_skills?.includes(skill.id) ??
                      false
                    }
                    onChange={() =>
                      updateRole(currentRole, {
                        required_skills: toggleListValue(
                          currentRoleConfig.required_skills ?? [],
                          skill.id,
                        ),
                      })
                    }
                    disabled={saving}
                  />
                  <span>{skill.name || skill.id}</span>
                  <small>{skill.risk || "low"}</small>
                </label>
              ))}
            </div>
          </div>
        </div>
      ) : null}
      <div className="config-preview">
        <div className="mini-heading">
          <strong>Graph preview</strong>
          <span>{hasDiff ? "changed" : "saved"}</span>
        </div>
        <div className="graph-preview">
          {roleOrder
            .filter((roleID) => !disabled.has(roleID))
            .map((roleID) => draft.roles?.[roleID]?.purpose || roleID)
            .join(" -> ") || "No enabled roles"}
        </div>
        <div className="mini-heading">
          <strong>Pending config</strong>
          <span>{saving ? "saving" : "local draft"}</span>
        </div>
        <pre>{JSON.stringify(draft, null, 2)}</pre>
      </div>
      <button
        className="button"
        type="button"
        disabled={saving}
        onClick={() => onSave({ ...draft, role_order: roleOrder })}
      >
        Save role flow
      </button>
    </section>
  );
}

function AgentBuilder({
  agents,
  models,
  graphSnapshot,
  toolCatalog,
  skillCatalog,
  draft,
  setDraft,
  saving,
  validating,
  validation,
  onValidate,
  onSave,
}: {
  agents: AgentRecord[];
  models: ProviderProfile[];
  graphSnapshot?: GraphSnapshot;
  toolCatalog: ToolDefinition[];
  skillCatalog: SkillDefinition[];
  draft: AgentRecord;
  setDraft: (next: AgentRecord) => void;
  saving: boolean;
  validating: boolean;
  validation: string;
  onValidate: () => void;
  onSave: (event: FormEvent<HTMLFormElement>) => void;
}) {
  const graphModelOptions = graphSnapshot
    ? Object.entries(graphSnapshot.ir.models).map(([id, model]) => ({
        id,
        label: `${id} / ${model.model}`,
      }))
    : [];
  const modelOptions = graphModelOptions.length
    ? graphModelOptions
    : models.map((model) => ({
        id: model.id,
        label: `${model.name || model.id} / ${model.model}`,
      }));
  const agentIsInvalid =
    draft.id.trim() === "" ||
    ((draft.kind === "model_agent" || draft.kind === "gateway_agent") &&
      (draft.model ?? "").trim() === "") ||
    (draft.kind === "external_agent" && (draft.runtime ?? "").trim() === "");
  return (
    <section className="panel" aria-label="Agent builder">
      <div className="panel-heading">
        <div>
          <h2>Agent Builder</h2>
          <p>Create or update shared project agents</p>
        </div>
        <span className="tag">{agents.length}</span>
      </div>
      <form className="builder-form" onSubmit={onSave}>
        <div className="builder-grid">
          <label>
            <span>ID</span>
            <input
              value={draft.id}
              onChange={(event) =>
                setDraft({ ...draft, id: event.target.value })
              }
              placeholder="planner"
            />
          </label>
          <label>
            <span>Name</span>
            <input
              value={draft.name ?? ""}
              onChange={(event) =>
                setDraft({ ...draft, name: event.target.value })
              }
              placeholder="Research Agent"
            />
          </label>
          <label>
            <span>Kind</span>
            <select
              value={draft.kind}
              onChange={(event) =>
                setDraft({ ...draft, kind: event.target.value })
              }
            >
              <option value="model_agent">Model agent</option>
              <option value="gateway_agent">Gateway agent</option>
              <option value="external_agent">External agent</option>
            </select>
          </label>
          <label>
            <span>Model</span>
            <select
              value={draft.model ?? ""}
              onChange={(event) =>
                setDraft({ ...draft, model: event.target.value })
              }
            >
              <option value="">Select model profile</option>
              {modelOptions.map((model) => (
                <option value={model.id} key={model.id}>
                  {model.label}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span>Runtime</span>
            <input
              value={draft.runtime ?? ""}
              onChange={(event) =>
                setDraft({ ...draft, runtime: event.target.value })
              }
              placeholder="local_cli"
            />
          </label>
          <label>
            <span>Approval policy</span>
            <select
              value={draft.approval_policy ?? "default"}
              onChange={(event) =>
                setDraft({ ...draft, approval_policy: event.target.value })
              }
            >
              <option value="default">Default</option>
              <option value="ask">Ask for mutations</option>
              <option value="strict">Strict review</option>
              <option value="readonly">Read-only</option>
            </select>
          </label>
        </div>
        {draft.kind === "external_agent" ? (
          <div className="builder-grid">
            <label>
              <span>Command template</span>
              <input
                value={String(draft.runtime_profile?.command_template ?? "")}
                onChange={(event) =>
                  setDraft({
                    ...draft,
                    runtime_profile: {
                      ...(draft.runtime_profile ?? {}),
                      command_template: event.target.value,
                    },
                  })
                }
                placeholder="agent-cli run --prompt {{prompt}}"
              />
            </label>
            <label>
              <span>Timeout seconds</span>
              <input
                type="number"
                min="1"
                max="3600"
                value={String(draft.runtime_profile?.timeout_seconds ?? "")}
                onChange={(event) =>
                  setDraft({
                    ...draft,
                    runtime_profile: {
                      ...(draft.runtime_profile ?? {}),
                      timeout_seconds: Number(event.target.value || 0),
                    },
                  })
                }
                placeholder="300"
              />
            </label>
          </div>
        ) : null}
        <div className="builder-grid">
          <label>
            <span>Filesystem permission</span>
            <select
              value={String(draft.permissions?.filesystem ?? "approval")}
              onChange={(event) =>
                setDraft({
                  ...draft,
                  permissions: {
                    ...(draft.permissions ?? {}),
                    filesystem: event.target.value,
                  },
                })
              }
            >
              <option value="read">Read</option>
              <option value="approval">Write with approval</option>
              <option value="none">None</option>
            </select>
          </label>
          <label>
            <span>Bash permission</span>
            <select
              value={String(draft.permissions?.bash ?? "approval")}
              onChange={(event) =>
                setDraft({
                  ...draft,
                  permissions: {
                    ...(draft.permissions ?? {}),
                    bash: event.target.value,
                  },
                })
              }
            >
              <option value="approval">Run with approval</option>
              <option value="none">None</option>
            </select>
          </label>
        </div>
        <label>
          <span>Description</span>
          <input
            value={draft.description ?? ""}
            onChange={(event) =>
              setDraft({ ...draft, description: event.target.value })
            }
            placeholder="When this agent should be selected"
          />
        </label>
        <label>
          <span>Role</span>
          <input
            value={draft.role ?? ""}
            onChange={(event) =>
              setDraft({ ...draft, role: event.target.value })
            }
            placeholder="Plan and coordinate workspace runs"
          />
        </label>
        <label>
          <span>Instructions</span>
          <textarea
            rows={4}
            value={draft.instructions ?? ""}
            onChange={(event) =>
              setDraft({ ...draft, instructions: event.target.value })
            }
            placeholder="Operating instructions"
          />
        </label>
        <div className="builder-grid">
          <label>
            <span>Triggers</span>
            <input
              value={draft.triggers?.join(", ") ?? ""}
              onChange={(event) =>
                setDraft({ ...draft, triggers: splitCSV(event.target.value) })
              }
              placeholder="investigate, implement, verify"
            />
          </label>
          <label>
            <span>Tags</span>
            <input
              value={draft.tags?.join(", ") ?? ""}
              onChange={(event) =>
                setDraft({ ...draft, tags: splitCSV(event.target.value) })
              }
              placeholder="project, local"
            />
          </label>
        </div>
        <div className="selection-panel">
          <div className="mini-heading no-border">
            <strong>Tools</strong>
            <span>{draft.tools?.length ?? 0} selected</span>
          </div>
          <div className="checkbox-grid">
            {toolCatalog.map((tool) => (
              <label key={tool.id}>
                <input
                  type="checkbox"
                  checked={draft.tools?.includes(tool.id) ?? false}
                  onChange={() =>
                    setDraft({
                      ...draft,
                      tools: toggleListValue(draft.tools ?? [], tool.id),
                    })
                  }
                />
                <span>{tool.id}</span>
                <small>{tool.mutation_risk}</small>
              </label>
            ))}
          </div>
        </div>
        <div className="selection-panel">
          <div className="mini-heading no-border">
            <strong>Skills</strong>
            <span>{draft.skills?.length ?? 0} selected</span>
          </div>
          <div className="checkbox-grid">
            {skillCatalog.map((skill) => (
              <label key={skill.id}>
                <input
                  type="checkbox"
                  checked={draft.skills?.includes(skill.id) ?? false}
                  onChange={() =>
                    setDraft({
                      ...draft,
                      skills: toggleListValue(draft.skills ?? [], skill.id),
                    })
                  }
                />
                <span>{skill.name || skill.id}</span>
                <small>{skill.risk || "low"}</small>
              </label>
            ))}
          </div>
        </div>
        {validation ? (
          <div
            className={
              validation === "valid" ? "inline-success" : "inline-error"
            }
          >
            {validation === "valid" ? "Agent config is valid." : validation}
          </div>
        ) : null}
        <div className="config-preview compact-config">
          <div className="mini-heading no-border">
            <strong>Save preview</strong>
            <span>{draft.id || "new agent"}</span>
          </div>
          <pre>{JSON.stringify(draft, null, 2)}</pre>
        </div>
        <button
          className="button button-secondary"
          type="button"
          disabled={validating || agentIsInvalid}
          onClick={onValidate}
        >
          {validating ? "Validating" : "Validate"}
        </button>
        <button
          className="button"
          type="submit"
          disabled={saving || agentIsInvalid}
        >
          {saving ? "Saving" : "Save agent"}
        </button>
      </form>
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
      return {
        supported: false,
        reason: "handoff chain has multiple outgoing edges",
      };
    }
    const edge = outgoing[0];
    if (edge.mode !== "handoff") {
      return { supported: false, reason: "only handoff chains are executable" };
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

function latestRouteDecision(messages: ChatMessage[]): RouteDecision | null {
  for (const message of [...messages].reverse()) {
    if (message.metadata?.route_decision) {
      return message.metadata.route_decision;
    }
  }
  return null;
}

function splitCSV(value: string): string[] {
  return value
    .split(",")
    .map((part) => part.trim())
    .filter(Boolean);
}

function toggleListValue(values: string[], value: string): string[] {
  return values.includes(value)
    ? values.filter((current) => current !== value)
    : [...values, value];
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

function readTheme(): Theme {
  const saved = window.localStorage.getItem("nomici.console.theme");
  if (saved === "light" || saved === "dark") {
    return saved;
  }
  return window.matchMedia("(prefers-color-scheme: light)").matches
    ? "light"
    : "dark";
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
