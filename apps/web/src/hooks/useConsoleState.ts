import { type FormEvent, useEffect, useMemo, useState } from "react";
import { apiRequest as gatewayRequest } from "../api/client";
import {
  emptyOverview,
  type AgentRecord,
  type AgentTemplate,
  type AgentTestResult,
  type Approval,
  type ArtifactContent,
  type ArtifactRecord,
  type ArtifactRevision,
  type BlockedAction,
  type ChatDetail,
  type ChatMessageResponse,
  type ChatThread,
  type ContextUsageItem,
  type FeatureReadiness,
  type MemoryItem,
  type MemoryProposal,
  type OrchestrationConfig,
  type OrchestrationPreview,
  type Overview,
  type ProviderDefinition,
  type ProviderProfile,
  type RouteDecision,
  type RunSessionDetail,
  type SkillDefinition,
  type Theme,
  type TimelineItem,
  type TodoItem,
  type ToolDefinition,
  type TokenUsage,
  type TraceEvent,
  type UploadRecord,
  type View,
} from "../api/types";
import { buildAgentOptions } from "../lib/agents";
import {
  latestRouteDecision,
  mergeEvents,
  normalizeOverview,
  readTheme,
} from "../lib/format";
import { splitCSV } from "../lib/lists";
import { useSessionEvents } from "./useSessionEvents";

export function useConsoleState() {
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
  const [featureReadiness, setFeatureReadiness] = useState<
    FeatureReadiness[]
  >([]);
  const [toolCatalog, setToolCatalog] = useState<ToolDefinition[]>([]);
  const [skillCatalog, setSkillCatalog] = useState<SkillDefinition[]>([]);
  const [agents, setAgents] = useState<AgentRecord[]>([]);
  const [agentTemplates, setAgentTemplates] = useState<AgentTemplate[]>([]);
  const [agentTestResult, setAgentTestResult] =
    useState<AgentTestResult | null>(null);
  const [orchestration, setOrchestration] = useState<OrchestrationConfig>({});
  const [orchestrationPreview, setOrchestrationPreview] =
    useState<OrchestrationPreview | null>(null);
  const [chats, setChats] = useState<ChatThread[]>([]);
  const [chatDetail, setChatDetail] = useState<ChatDetail | null>(null);
  const [chatSuggestions, setChatSuggestions] = useState<string[]>([]);
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
  const [sessionTimeline, setSessionTimeline] = useState<TimelineItem[]>([]);
  const [sessionTodos, setSessionTodos] = useState<TodoItem[]>([]);
  const [sessionApprovals, setSessionApprovals] = useState<Approval[]>([]);
  const [sessionContextUsage, setSessionContextUsage] = useState<
    ContextUsageItem[]
  >([]);
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
  const [skillDraft, setSkillDraft] = useState<SkillDefinition>({
    id: "",
    name: "",
    description: "",
    triggers: [],
    required_tools: [],
    risk: "low",
    compatibility: "local",
    briefing: "",
    enabled: true,
  });
  const [skillImportPath, setSkillImportPath] = useState("");
  const [settingsMutation, setSettingsMutation] = useState("");
  const [agentValidation, setAgentValidation] = useState("");

  const isAuthenticated = status === "ready";
  const agentOptions = useMemo(
    () => buildAgentOptions(overview.graph_snapshot),
    [overview.graph_snapshot],
  );
  const selectedAgent = agentOptions.find((agent) => agent.id === runAgentId);
  const traceEvents = runEvents.length > 0 ? runEvents : overview.latest_trace;
  const activeModelLabel = useMemo(
    () => modelConnectionLabel(overview.models),
    [overview.models],
  );
  const hasConfiguredModel = overview.models.length > 0;
  const tokenUsage = useMemo(
    () => aggregateTokenUsage(traceEvents),
    [traceEvents],
  );
  const enabledSkillIds = useMemo(
    () =>
      new Set(
        skillCatalog
          .filter((skill) => skill.enabled !== false)
          .map((skill) => skill.id),
      ),
    [skillCatalog],
  );
  const workspaceTasks = sessionDetail?.tasks ?? [];
  const workspaceUploads = sessionDetail?.uploads ?? [];
  const workspaceArtifacts = sessionDetail?.artifacts ?? [];
  const workspaceToolCalls = sessionDetail?.tool_calls ?? [];
  const planArtifact = workspaceArtifacts.find(
    (artifact) => artifact.type === "plan",
  );
  const sessionNeedsPlanReview =
    sessionDetail?.session.status === "plan_review" && planArtifact;
  const openBlockedActionCount =
    sessionDetail?.blocked_actions?.filter((action) => action.status === "open")
      .length ?? 0;
  const hasWorkspaceActivity =
    activeRunId !== "" ||
    activeSessionId !== "" ||
    sessionDetail !== null;
  const hasVisibleWorkspaceWork =
    workspaceArtifacts.length > 0 ||
    workspaceToolCalls.length > 0 ||
    sessionApprovals.length > 0 ||
    sessionContextUsage.length > 0;
  const hasActionableWorkspace =
    openBlockedActionCount > 0 ||
    sessionDetail?.session.status === "running" ||
    sessionDetail?.session.status === "plan_review" ||
    sessionDetail?.session.status === "blocked" ||
    sessionDetail?.session.status === "needs_clarification";
  const showChatWorkspace =
    view === "chat" &&
    Boolean(sessionDetail && (hasActionableWorkspace || hasVisibleWorkspaceWork));
  const readinessById = useMemo(() => {
    const next = new Map<string, FeatureReadiness>();
    featureReadiness.forEach((item) => next.set(item.id, item));
    return next;
  }, [featureReadiness]);
  const featureWorks = (id: string) => readinessById.get(id)?.status === "works";
  const featureReason = (id: string) => readinessById.get(id)?.reason ?? "";

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

  useSessionEvents({
    activeRunId,
    activeSessionId,
    runEvents,
    runStatus,
    gatewayToken,
    request,
    loadSessionDetail,
    loadOverview,
    setRunEvents,
    setRunStatus,
    setRunError,
  });

  async function loadOverview(nextToken = gatewayToken) {
    const bootstrapToken = readBootstrapTokenFromLocation();
    if (bootstrapToken !== "" && nextToken === gatewayToken) {
      await redeemBootstrapToken(bootstrapToken);
      return;
    }
    setStatus("loading");
    setError("");
    try {
      const nextOverview = await request<Overview>(
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
        nextAgentTemplates,
        nextOrchestration,
        nextMemory,
        nextMemoryItems,
        nextReviewQueue,
        nextReadiness,
      ] = await Promise.all([
        request<ChatThread[]>("/api/chats?limit=50", {}, nextToken),
        request<ProviderDefinition[]>("/api/provider-catalog", {}, nextToken),
        request<ToolDefinition[]>("/api/tools", {}, nextToken).catch(() => []),
        request<SkillDefinition[]>("/api/skills", {}, nextToken).catch(
          () => [],
        ),
        request<AgentRecord[]>("/api/agents", {}, nextToken).catch(() => []),
        request<AgentTemplate[]>("/api/agents/templates", {}, nextToken).catch(
          () => [],
        ),
        request<OrchestrationConfig>("/api/orchestration", {}, nextToken).catch(
          () => ({}),
        ),
        request<MemoryProposal[]>(
          "/api/memory/proposals?status=proposed",
          {},
          nextToken,
        ).catch(() => []),
        request<MemoryItem[]>("/api/memory/items", {}, nextToken).catch(
          () => [],
        ),
        request<BlockedAction[]>(
          "/api/review-queue?status=open",
          {},
          nextToken,
        ).catch(() => []),
        request<FeatureReadiness[]>(
          "/api/features/readiness",
          {},
          nextToken,
        ).catch(() => []),
      ]);
      setChats(nextChats ?? []);
      setProviderCatalog(catalog ?? []);
      setToolCatalog(nextTools ?? []);
      setSkillCatalog(nextSkills ?? []);
      setAgents(nextAgents ?? []);
      setAgentTemplates(nextAgentTemplates ?? []);
      setOrchestration(nextOrchestration ?? {});
      setMemoryProposals(nextMemory ?? []);
      setMemoryItems(nextMemoryItems ?? []);
      setReviewQueue(nextReviewQueue ?? []);
      setFeatureReadiness(nextReadiness ?? []);
      setStatus("ready");
    } catch (loadError) {
      const message =
        loadError instanceof Error ? loadError.message : "Gateway unavailable";
      if (message.includes("token")) {
        window.localStorage.removeItem("nomici.gateway.token");
        setGatewayToken("");
        setTokenInput("");
        setStatus("auth");
      } else {
        setStatus("failed");
      }
      setError(message);
    }
  }

  async function redeemBootstrapToken(bootstrapToken: string) {
    setStatus("loading");
    setError("");
    try {
      const response = await gatewayRequest<{ gateway_token: string }>(
        "/api/auth/bootstrap",
        {
          method: "POST",
          body: JSON.stringify({ token: bootstrapToken }),
        },
      );
      const nextToken = response.gateway_token.trim();
      if (nextToken === "") {
        throw new Error("Gateway returned an empty bootstrap token response");
      }
      window.localStorage.setItem("nomici.gateway.token", nextToken);
      setGatewayToken(nextToken);
      setTokenInput(nextToken);
      clearBootstrapTokenFromLocation();
      await loadOverview(nextToken);
    } catch (bootstrapError) {
      clearBootstrapTokenFromLocation();
      window.localStorage.removeItem("nomici.gateway.token");
      setGatewayToken("");
      setTokenInput("");
      setStatus("auth");
      setError(
        bootstrapError instanceof Error
          ? bootstrapError.message
          : "Gateway bootstrap failed",
      );
    }
  }

  async function request<T>(
    path: string,
    init: RequestInit = {},
    tokenOverride?: string,
  ): Promise<T> {
    return gatewayRequest<T>(
      path,
      init,
      tokenOverride ?? gatewayToken,
      path === "/api/console/overview" ? setWarnings : undefined,
    );
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

  async function reconnectLocalGateway() {
    setStatus("loading");
    setError("");
    try {
      const response = await gatewayRequest<{ gateway_token: string }>(
        "/api/auth/reconnect",
        { method: "POST" },
      );
      const nextToken = response.gateway_token.trim();
      if (nextToken === "") {
        throw new Error("Gateway returned an empty reconnect response");
      }
      window.localStorage.setItem("nomici.gateway.token", nextToken);
      setGatewayToken(nextToken);
      setTokenInput(nextToken);
      await loadOverview(nextToken);
    } catch (reconnectError) {
      window.localStorage.removeItem("nomici.gateway.token");
      setGatewayToken("");
      setTokenInput("");
      setStatus("auth");
      setError(
        reconnectError instanceof Error
          ? reconnectError.message
          : "Local Gateway reconnect failed",
      );
    }
  }

  async function selectChat(chatID: string) {
    setView("chat");
    const detail = await request<ChatDetail>(
      `/api/chats/${encodeURIComponent(chatID)}`,
    );
    setChatDetail(detail);
    void loadChatSuggestions(chatID);
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
      setActiveRunId("");
      setActiveSessionId("");
      setSessionDetail(null);
      setRunEvents([]);
      setRunStatus("idle");
      setSessionApprovals([]);
      setSessionContextUsage([]);
      setActiveRouteDecision(latestRouteDecision(detail.messages));
    }
  }

  function startNewChat() {
    setView("chat");
    setChatDetail(null);
    setMessageText("");
    setRunStatus("idle");
    setRunError("");
    setWorkspaceError("");
    setActiveRouteDecision(null);
    setActiveRunId("");
    setActiveSessionId("");
    setSessionDetail(null);
    setRunEvents([]);
    setChatSuggestions([]);
    setSessionTimeline([]);
    setSessionTodos([]);
    setSessionApprovals([]);
    setSessionContextUsage([]);
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
      const response = await request<ChatMessageResponse>(path, {
        method: "POST",
        body: JSON.stringify({
          agent_id: runAgentId === "auto" ? "" : selectedAgent?.id,
          selected_skills: selectedSkillIds.filter((id) =>
            enabledSkillIds.has(id),
          ),
          prompt: content,
          content,
        }),
      });
      setActiveRouteDecision(response.route_decision ?? null);
      setMessageText("");
      if (response.provider_error) {
        const provider = response.provider_error.provider_id
          ? ` (${response.provider_error.provider_id}${response.provider_error.model_id ? ` / ${response.provider_error.model_id}` : ""})`
          : "";
        setRunStatus("idle");
        setRunError(
          `${response.provider_error.message}${provider}${
            response.provider_error.remediation
              ? ` ${response.provider_error.remediation}`
              : ""
          }`,
        );
        setActiveRunId("");
        setActiveSessionId("");
        setSessionDetail(null);
        setRunEvents([]);
        setSessionApprovals([]);
        setSessionContextUsage([]);
      } else if (response.run) {
        setActiveRunId(response.run.run_id);
        setActiveSessionId(response.run.session_id ?? "");
        setRunStatus("running");
        if (response.run.session_id) {
          await loadSessionDetail(response.run.session_id);
        }
      } else {
        setActiveRunId("");
        setActiveSessionId("");
        setSessionDetail(null);
        setRunEvents([]);
        setRunStatus("idle");
        setSessionApprovals([]);
        setSessionContextUsage([]);
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
      request<ChatThread[]>("/api/chats?limit=50"),
      request<ChatDetail>(`/api/chats/${encodeURIComponent(chatID)}`),
    ]);
    setChats(nextChats ?? []);
    setChatDetail(detail);
    setActiveRouteDecision(latestRouteDecision(detail.messages));
    void loadChatSuggestions(chatID);
  }

  async function loadSessionDetail(sessionId: string) {
    if (!sessionId) {
      return;
    }
    const detail = await request<RunSessionDetail>(
      `/api/sessions/${encodeURIComponent(sessionId)}`,
    );
    setSessionDetail(detail);
    if (detail.session.status === "completed") {
      setRunStatus("completed");
    } else if (
      detail.session.status === "failed" ||
      detail.session.status === "cancelled"
    ) {
      setRunStatus("failed");
    } else {
      setRunStatus("running");
    }
    setActiveRouteDecision(detail.session.metadata?.route_decision ?? null);
    const [timeline, todos, approvals, contextUsage] = await Promise.all([
      request<TimelineItem[]>(
        `/api/sessions/${encodeURIComponent(sessionId)}/timeline`,
      ).catch(() => []),
      request<TodoItem[]>(
        `/api/sessions/${encodeURIComponent(sessionId)}/todos`,
      ).catch(() => []),
      request<Approval[]>(
        `/api/sessions/${encodeURIComponent(sessionId)}/approvals?status=pending`,
      ).catch(() => []),
      request<ContextUsageItem[]>(
        `/api/sessions/${encodeURIComponent(sessionId)}/context-usage`,
      ).catch(() => []),
    ]);
    setSessionTimeline(timeline ?? []);
    setSessionTodos(todos ?? []);
    setSessionApprovals(approvals ?? []);
    setSessionContextUsage(contextUsage ?? []);
  }

  async function loadChatSuggestions(chatID: string) {
    const suggestions = await request<string[]>(
      `/api/chats/${encodeURIComponent(chatID)}/suggestions`,
    ).catch(() => []);
    setChatSuggestions(suggestions ?? []);
  }

  async function submitMessageFeedback(messageID: string, score: string) {
    if (!chatDetail) {
      return;
    }
    await request(
      `/api/chats/${encodeURIComponent(chatDetail.thread.chat_id)}/feedback`,
      {
        method: "POST",
        body: JSON.stringify({ message_id: messageID, score }),
      },
    ).catch(() => null);
  }

  async function approvePlan() {
    if (!activeSessionId || !planArtifact) {
      return;
    }
    setWorkspaceError("");
    setWorkspaceMutation("approve-plan");
    try {
      const detail = await request<RunSessionDetail>(
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
      await request<ArtifactRecord>(
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
      const detail = await request<RunSessionDetail>(
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
      await request<UploadRecord>("/api/uploads", { method: "POST", body });
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
      await request<Approval>(
        `/api/approvals/${encodeURIComponent(approvalID)}/${action}`,
        {
          method: "POST",
          body:
            action === "grant" ? JSON.stringify({ scope: "run" }) : undefined,
        },
      );
      if (action === "grant" && activeSessionId) {
        const detail = await request<RunSessionDetail>(
          `/api/sessions/${encodeURIComponent(activeSessionId)}/resume`,
          { method: "POST" },
        ).catch(() => null);
        if (detail) {
          setSessionDetail(detail);
          setRunStatus("running");
        }
      }
      await loadOverview();
      if (activeSessionId) {
        await loadSessionDetail(activeSessionId);
      }
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
      await request<MemoryProposal>(
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
      await request<{ status: string }>(
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
      const detail = await request<RunSessionDetail>(
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
      const content = await request<ArtifactContent>(
        `/api/artifacts/${encodeURIComponent(artifactID)}/content`,
      );
      const revisions = await request<ArtifactRevision[]>(
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
      const detail = await request<RunSessionDetail>(
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
      await request<{ status: string }>(
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

  function applyAgentTemplate(template: AgentTemplate) {
    setAgentDraft({
      id: template.id,
      name: template.name,
      description: template.description,
      kind: template.kind,
      role: template.role,
      instructions: template.instructions,
      tools: template.tools ?? [],
      skills: template.skills ?? [],
      tags: template.tags ?? [],
      triggers: template.triggers ?? [],
      permissions: template.permissions,
      approval_policy: template.approval_policy,
    });
    setAgentValidation("");
    setAgentTestResult(null);
    setView("agents");
  }

  function resetAgentDraft() {
    setAgentDraft({
      id: "",
      kind: "model_agent",
      model: overview.models[0]?.id ?? "",
      role: "",
      instructions: "",
      tools: [],
      skills: [],
      tags: [],
      triggers: [],
    });
    setAgentValidation("");
    setAgentTestResult(null);
    setView("agents");
  }

  function draftAgentFromChat() {
    const messages = chatDetail?.messages ?? [];
    const lastUser = [...messages]
      .reverse()
      .find((message) => message.role === "user");
    const seed = lastUser?.content || messageText || "Custom workspace agent";
    const words = seed
      .toLowerCase()
      .replace(/[^a-z0-9\s_-]/g, "")
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 4);
    const id = (words.join("_") || "custom_agent").replace(
      /^([0-9])/,
      "agent_$1",
    );
    setAgentDraft({
      id,
      name: titleCase(words.join(" ") || "Custom Agent"),
      description: seed.slice(0, 160),
      kind: "model_agent",
      model: overview.models[0]?.id ?? "",
      role: "Handle this recurring workspace workflow.",
      instructions: `Use this agent when the user asks for work similar to:\n${seed}`,
      tools: [],
      skills: selectedSkillIds,
      tags: ["custom"],
      triggers: words,
    });
    setAgentValidation("");
    setAgentTestResult(null);
    setView("agents");
  }

  function exportChat() {
    const payload = {
      exported_at: new Date().toISOString(),
      chat: chatDetail,
      run_id: activeRunId,
      session: sessionDetail,
      token_usage: tokenUsage,
    };
    const blob = new Blob([JSON.stringify(payload, null, 2)], {
      type: "application/json",
    });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `${chatDetail?.thread.title || "nomici-chat"}.json`;
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  }

  async function testAgentDraft() {
    if (agentDraft.id.trim() === "") {
      setAgentValidation("Agent id is required.");
      return;
    }
    setSettingsMutation("agent-test");
    setAgentValidation("");
    setAgentTestResult(null);
    try {
      const result = await request<AgentTestResult>(
        `/api/agents/${encodeURIComponent(agentDraft.id)}/test`,
        {
          method: "POST",
          body: JSON.stringify({
            prompt: "Reply with one concise sentence confirming this agent is ready.",
            execute: true,
          }),
        },
      );
      setAgentTestResult(result);
      setAgentValidation(
        result.mode === "executed"
          ? result.status || "tested"
          : `Diagnostic only: ${result.truth_label || result.mode}`,
      );
    } catch (testError) {
      setAgentValidation(
        testError instanceof Error ? testError.message : "Agent test failed",
      );
    } finally {
      setSettingsMutation("");
    }
  }

  async function copyAgentToProject(agent: AgentRecord) {
    setSettingsMutation(`agent-copy:${agent.id}`);
    setRunError("");
    setAgentValidation("");
    try {
      const copied = await request<AgentRecord>(
        `/api/agents/${encodeURIComponent(agent.id)}/copy`,
        {
          method: "POST",
          body: JSON.stringify({}),
        },
      );
      setAgentDraft(copied);
      setAgentValidation(`${copied.id} copied to project config.`);
      await loadOverview();
    } catch (copyError) {
      setAgentValidation(
        copyError instanceof Error ? copyError.message : "Agent copy failed",
      );
    } finally {
      setSettingsMutation("");
    }
  }

  async function setAgentEnabled(agent: AgentRecord, enabled: boolean) {
    setSettingsMutation(`agent-${enabled ? "enable" : "disable"}:${agent.id}`);
    setAgentValidation("");
    try {
      const updated = await request<AgentRecord>(
        `/api/agents/${encodeURIComponent(agent.id)}/${enabled ? "enable" : "disable"}`,
        { method: "POST" },
      );
      if (agentDraft.id === agent.id) {
        setAgentDraft(updated);
      }
      await loadOverview();
    } catch (agentError) {
      setAgentValidation(
        agentError instanceof Error
          ? agentError.message
          : "Agent status could not be updated",
      );
    } finally {
      setSettingsMutation("");
    }
  }

  async function deleteAgent(agent: AgentRecord) {
    setSettingsMutation(`agent-delete:${agent.id}`);
    setAgentValidation("");
    try {
      await request<{ status: string }>(
        `/api/agents/${encodeURIComponent(agent.id)}`,
        { method: "DELETE" },
      );
      if (agentDraft.id === agent.id) {
        resetAgentDraft();
      }
      await loadOverview();
    } catch (agentError) {
      setAgentValidation(
        agentError instanceof Error
          ? agentError.message
          : "Agent could not be deleted",
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
      await request<AgentRecord>("/api/agents", {
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

  async function saveSkillDraft(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (skillDraft.id.trim() === "") {
      setRunError("Skill id is required.");
      return;
    }
    setSettingsMutation("skill");
    setRunError("");
    try {
      await request<SkillDefinition>("/api/skills", {
        method: "POST",
        body: JSON.stringify({
          ...skillDraft,
          triggers: splitCSV(skillDraft.triggers?.join(",") ?? ""),
          required_tools: splitCSV(skillDraft.required_tools?.join(",") ?? ""),
          files: splitCSV(skillDraft.files?.join(",") ?? ""),
          enabled: skillDraft.enabled !== false,
        }),
      });
      setSkillDraft({
        id: "",
        name: "",
        description: "",
        triggers: [],
        required_tools: [],
        risk: "low",
        compatibility: "local",
        briefing: "",
        enabled: true,
      });
      await loadOverview();
    } catch (saveError) {
      setRunError(
        saveError instanceof Error
          ? saveError.message
          : "Skill could not be saved",
      );
    } finally {
      setSettingsMutation("");
    }
  }

  async function toggleSkillEnabled(skill: SkillDefinition) {
    setSettingsMutation(`skill:${skill.id}`);
    setRunError("");
    try {
      const enabled = skill.enabled === false;
      await request<SkillDefinition>(
        `/api/skills/${encodeURIComponent(skill.id)}`,
        {
          method: "PATCH",
          body: JSON.stringify({ enabled }),
        },
      );
      await loadOverview();
      if (!enabled) {
        setSelectedSkillIds((current) =>
          current.filter((id) => id !== skill.id),
        );
      }
    } catch (toggleError) {
      setRunError(
        toggleError instanceof Error
          ? toggleError.message
          : "Skill status could not be updated",
      );
    } finally {
      setSettingsMutation("");
    }
  }

  async function deleteSkill(skill: SkillDefinition) {
    setSettingsMutation(`skill-delete:${skill.id}`);
    setRunError("");
    try {
      await request<{ status: string }>(
        `/api/skills/${encodeURIComponent(skill.id)}`,
        { method: "DELETE" },
      );
      await loadOverview();
    } catch (deleteError) {
      setRunError(
        deleteError instanceof Error ? deleteError.message : "Skill delete failed",
      );
    } finally {
      setSettingsMutation("");
    }
  }

  async function importSkill(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (skillImportPath.trim() === "") {
      setRunError("Skill directory path is required.");
      return;
    }
    setSettingsMutation("skill-import");
    setRunError("");
    try {
      await request<SkillDefinition>("/api/skills/import", {
        method: "POST",
        body: JSON.stringify({ path: skillImportPath.trim() }),
      });
      setSkillImportPath("");
      await loadOverview();
    } catch (importError) {
      setRunError(
        importError instanceof Error ? importError.message : "Skill import failed",
      );
    } finally {
      setSettingsMutation("");
    }
  }

  async function saveOrchestration(next: OrchestrationConfig) {
    setSettingsMutation("orchestration");
    setRunError("");
    try {
      const saved = await request<OrchestrationConfig>("/api/orchestration", {
        method: "PATCH",
        body: JSON.stringify(next),
      });
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

  async function previewOrchestration() {
    setSettingsMutation("orchestration-preview");
    setRunError("");
    try {
      const preview = await request<OrchestrationPreview>(
        "/api/orchestration/preview",
        {
          method: "POST",
          body: JSON.stringify({
            prompt:
              messageText.trim() ||
              "Preview a long-horizon workspace task.",
            agent_id: runAgentId === "auto" ? "" : runAgentId,
          }),
        },
      );
      setOrchestrationPreview(preview);
    } catch (previewError) {
      setRunError(
        previewError instanceof Error
          ? previewError.message
          : "Orchestration preview failed",
      );
    } finally {
      setSettingsMutation("");
    }
  }

  async function testOrchestration() {
    setSettingsMutation("orchestration-test");
    setRunError("");
    try {
      const preview = await request<OrchestrationPreview>(
        "/api/orchestration/test",
        {
          method: "POST",
          body: JSON.stringify({
            prompt:
              messageText.trim() ||
              "Preview a long-horizon workspace task.",
            agent_id: runAgentId === "auto" ? "" : runAgentId,
          }),
        },
      );
      setOrchestrationPreview(preview);
      if (preview.run?.session_id) {
        setActiveRunId(preview.run.run_id);
        setActiveSessionId(preview.run.session_id);
        setActiveRouteDecision(
          preview.run.route_decision ?? preview.route_decision,
        );
        setView("runs");
        await loadSessionDetail(preview.run.session_id);
      }
    } catch (testError) {
      setRunError(
        testError instanceof Error
          ? testError.message
          : "Orchestration test failed",
      );
    } finally {
      setSettingsMutation("");
    }
  }

  return {
    gatewayToken,
    tokenInput,
    setTokenInput,
    theme,
    setTheme,
    view,
    setView,
    overview,
    featureReadiness,
    featureWorks,
    featureReason,
    providerCatalog,
    toolCatalog,
    skillCatalog,
    enabledSkillIds,
    agents,
    agentTemplates,
    agentTestResult,
    orchestration,
    orchestrationPreview,
    chats,
    chatDetail,
    setChatDetail,
    chatSuggestions,
    warnings,
    status,
    error,
    isAuthenticated,
    runAgentId,
    setRunAgentId,
    messageText,
    setMessageText,
    selectedSkillIds,
    setSelectedSkillIds,
    activeRouteDecision,
    activeRunId,
    setActiveRunId,
    activeSessionId,
    setActiveSessionId,
    sessionDetail,
    runEvents,
    runStatus,
    setRunStatus,
    runError,
    workspaceError,
    planRevision,
    setPlanRevision,
    uploadFile,
    setUploadFile,
    workspaceMutation,
    clarificationAnswer,
    setClarificationAnswer,
    mutatingApproval,
    memoryProposals,
    memoryItems,
    reviewQueue,
    sessionTimeline,
    sessionTodos,
    sessionApprovals,
    sessionContextUsage,
    mutatingMemory,
    artifactContent,
    artifactRevisions,
    artifactMutation,
    agentDraft,
    setAgentDraft,
    skillDraft,
    setSkillDraft,
    skillImportPath,
    setSkillImportPath,
    settingsMutation,
    agentValidation,
    agentOptions,
    selectedAgent,
    traceEvents,
    activeModelLabel,
    hasConfiguredModel,
    tokenUsage,
    workspaceTasks,
    workspaceUploads,
    workspaceArtifacts,
    workspaceToolCalls,
    planArtifact,
    sessionNeedsPlanReview,
    hasWorkspaceActivity,
    showChatWorkspace,
    loadOverview,
    submitToken,
    reconnectLocalGateway,
    selectChat,
    startNewChat,
    sendMessage,
    loadSessionDetail,
    loadChatSuggestions,
    submitMessageFeedback,
    approvePlan,
    revisePlan,
    cancelSession,
    uploadInput,
    resolveApproval,
    resolveMemory,
    deleteMemoryItem,
    resolveBlockedAction,
    inspectArtifact,
    downloadArtifact,
    submitClarification,
    validateAgentDraft,
    applyAgentTemplate,
    resetAgentDraft,
    draftAgentFromChat,
    exportChat,
    testAgentDraft,
    copyAgentToProject,
    setAgentEnabled,
    deleteAgent,
    saveAgent,
    saveSkillDraft,
    toggleSkillEnabled,
    deleteSkill,
    importSkill,
    saveOrchestration,
    previewOrchestration,
    testOrchestration,
  };
}

export type ConsoleState = ReturnType<typeof useConsoleState>;

function readBootstrapTokenFromLocation(): string {
  const hash = window.location.hash.startsWith("#")
    ? window.location.hash.slice(1)
    : window.location.hash;
  const hashToken = new URLSearchParams(hash).get("bootstrap_token");
  if (hashToken) {
    return hashToken;
  }
  return new URLSearchParams(window.location.search).get("bootstrap_token") ?? "";
}

function clearBootstrapTokenFromLocation() {
  const url = new URL(window.location.href);
  url.searchParams.delete("bootstrap_token");
  url.hash = "";
  window.history.replaceState(null, "", `${url.pathname}${url.search}`);
}

function aggregateTokenUsage(events: TraceEvent[]): TokenUsage {
  return events.reduce(
    (usage, event) => {
      const raw = event.payload?.usage;
      if (!raw || typeof raw !== "object") {
        return usage;
      }
      const record = raw as Record<string, unknown>;
      const input =
        numberValue(record.prompt_tokens) || numberValue(record.input_tokens);
      const output =
        numberValue(record.completion_tokens) ||
        numberValue(record.output_tokens);
      const total = numberValue(record.total_tokens) || input + output;
      return {
        input: usage.input + input,
        output: usage.output + output,
        total: usage.total + total,
      };
    },
    { input: 0, output: 0, total: 0 },
  );
}

function numberValue(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function titleCase(value: string): string {
  return value
    .split(/\s+/)
    .filter(Boolean)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");
}

function modelConnectionLabel(models: ProviderProfile[]): string {
  if (!models.length) {
    return "No model configured";
  }
  const model = models[0];
  const provider = providerLabel(model.kind);
  if (provider) {
    return `${provider} / ${model.model || model.name || model.id}`;
  }
  return `${model.name || model.id} / ${model.model || model.kind}`;
}

function providerLabel(kind: string): string {
  switch (kind) {
    case "codex_cli":
      return "Local Codex CLI";
    case "claude_code":
      return "Claude Code OAuth";
    case "ollama":
      return "Local Ollama";
    case "anthropic":
      return "Anthropic";
    case "gemini":
      return "Google Gemini";
    case "openai_compatible":
      return "OpenAI-compatible";
    default:
      return titleCase(kind.replaceAll("_", " "));
  }
}
