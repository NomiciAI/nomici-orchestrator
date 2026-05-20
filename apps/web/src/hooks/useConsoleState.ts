import { type FormEvent, useEffect, useMemo, useState } from "react";
import { apiRequest as gatewayRequest } from "../api/client";
import {
  emptyOverview,
  type AgentRecord,
  type Approval,
  type ArtifactContent,
  type ArtifactRecord,
  type ArtifactRevision,
  type BlockedAction,
  type ChatDetail,
  type ChatMessageResponse,
  type ChatThread,
  type MemoryItem,
  type MemoryProposal,
  type OrchestrationConfig,
  type Overview,
  type ProviderDefinition,
  type RouteDecision,
  type RunSessionDetail,
  type SkillDefinition,
  type Theme,
  type ToolDefinition,
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
        nextOrchestration,
        nextMemory,
        nextMemoryItems,
        nextReviewQueue,
      ] = await Promise.all([
        request<ChatThread[]>("/api/chats?limit=50", {}, nextToken),
        request<ProviderDefinition[]>("/api/provider-catalog", {}, nextToken),
        request<ToolDefinition[]>("/api/tools", {}, nextToken).catch(() => []),
        request<SkillDefinition[]>("/api/skills", {}, nextToken).catch(
          () => [],
        ),
        request<AgentRecord[]>("/api/agents", {}, nextToken).catch(() => []),
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

  async function selectChat(chatID: string) {
    setView("chat");
    const detail = await request<ChatDetail>(
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
      const response = await request<ChatMessageResponse>(path, {
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
      request<ChatThread[]>("/api/chats?limit=50"),
      request<ChatDetail>(`/api/chats/${encodeURIComponent(chatID)}`),
    ]);
    setChats(nextChats ?? []);
    setChatDetail(detail);
    setActiveRouteDecision(latestRouteDecision(detail.messages));
  }

  async function loadSessionDetail(sessionId: string) {
    if (!sessionId) {
      return;
    }
    const detail = await request<RunSessionDetail>(
      `/api/sessions/${encodeURIComponent(sessionId)}`,
    );
    setSessionDetail(detail);
    setActiveRouteDecision(detail.session.metadata?.route_decision ?? null);
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

  return {
    gatewayToken,
    tokenInput,
    setTokenInput,
    theme,
    setTheme,
    view,
    setView,
    overview,
    providerCatalog,
    toolCatalog,
    skillCatalog,
    agents,
    orchestration,
    chats,
    chatDetail,
    setChatDetail,
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
    mutatingMemory,
    artifactContent,
    artifactRevisions,
    artifactMutation,
    agentDraft,
    setAgentDraft,
    settingsMutation,
    agentValidation,
    agentOptions,
    selectedAgent,
    traceEvents,
    workspaceTasks,
    workspaceUploads,
    workspaceArtifacts,
    workspaceToolCalls,
    planArtifact,
    sessionNeedsPlanReview,
    loadOverview,
    submitToken,
    selectChat,
    sendMessage,
    loadSessionDetail,
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
    saveAgent,
    saveOrchestration,
  };
}

export type ConsoleState = ReturnType<typeof useConsoleState>;
