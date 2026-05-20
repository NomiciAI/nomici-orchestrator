export type Theme = "dark" | "light";
export type View = "chat" | "orchestrate" | "settings";

export type ApiEnvelope<T> = {
  data: T;
  warnings: string[];
  request_id: string;
};

export type ApiErrorEnvelope = {
  error?: {
    code: string;
    message: string;
    remediation?: string;
  };
};

export type ProviderProfile = {
  id: string;
  name: string;
  kind: string;
  base_url: string;
  model: string;
  api_key_env: string;
  capabilities?: Record<string, string>;
};

export type ProviderDefinition = {
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

export type GraphSnapshot = {
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

export type RouteDecision = {
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

export type ToolStatus = {
  id: string;
  kind: string;
  provider: string;
  mode: string;
  status: string;
  auth: string;
  execution: string;
};

export type ToolDefinition = {
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

export type SkillDefinition = {
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

export type RunSession = {
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

export type RunTask = {
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

export type SandboxRecord = {
  sandbox_id: string;
  provider: string;
  mode: string;
  status: string;
  workspace_root?: string;
  artifact_root?: string;
  cleanup_status: string;
};

export type UploadRecord = {
  upload_id: string;
  filename: string;
  path: string;
  size_bytes: number;
  status: string;
  created_at: string;
};

export type ArtifactRecord = {
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

export type ArtifactContent = {
  artifact_id: string;
  path: string;
  content: string;
  truncated: boolean;
};

export type ArtifactRevision = {
  revision_id: string;
  artifact_id: string;
  revision: number;
  review_state: string;
  diff_preview?: string;
  created_at: string;
};

export type ToolCallRecord = {
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

export type BlockedAction = {
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

export type RunSessionDetail = {
  session: RunSession;
  tasks: RunTask[];
  sandbox?: SandboxRecord;
  uploads?: UploadRecord[];
  artifacts?: ArtifactRecord[];
  tool_calls?: ToolCallRecord[];
  blocked_actions?: BlockedAction[];
};

export type TraceEvent = {
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

export type Approval = {
  approval_id: string;
  run_id: string;
  status: string;
  risk: string;
  summary: string;
  requested_by_agent?: string;
};

export type MemoryProposal = {
  proposal_id: string;
  title: string;
  body: string;
  status: string;
  context_id?: string;
  updated_at: string;
};

export type MemoryItem = {
  context_id: string;
  title: string;
  body: string;
  tags?: string[];
  artifact_refs?: string[];
  updated_at: string;
};

export type ChatThread = {
  chat_id: string;
  title: string;
  status: string;
  updated_at: string;
};

export type ChatMessage = {
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

export type ChatDetail = {
  thread: ChatThread;
  messages: ChatMessage[];
};

export type ChatMessageResponse = {
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

export type AgentRecord = {
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

export type OrchestrationConfig = {
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

export type Overview = {
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

export const emptyOverview: Overview = {
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
