package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/orchestration"
	"github.com/NomiciAI/nomici-orchestrator/internal/projectconfig"
	"github.com/NomiciAI/nomici-orchestrator/internal/runs"
	"github.com/go-chi/chi/v5"
)

type agentTemplate struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Kind           string         `json:"kind"`
	Role           string         `json:"role"`
	Instructions   string         `json:"instructions"`
	Tools          []string       `json:"tools,omitempty"`
	Skills         []string       `json:"skills,omitempty"`
	Tags           []string       `json:"tags,omitempty"`
	Triggers       []string       `json:"triggers,omitempty"`
	Permissions    map[string]any `json:"permissions,omitempty"`
	ApprovalPolicy string         `json:"approval_policy,omitempty"`
}

type agentTestRequest struct {
	Prompt  string `json:"prompt"`
	Execute *bool  `json:"execute,omitempty"`
}

type agentTestResponse struct {
	AgentID   string   `json:"agent_id"`
	Status    string   `json:"status"`
	Mode      string   `json:"mode"`
	RunID     string   `json:"run_id,omitempty"`
	Output    string   `json:"output,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	TraceHint string   `json:"trace_hint,omitempty"`
}

type orchestrationPreviewRequest struct {
	Prompt  string `json:"prompt"`
	AgentID string `json:"agent_id"`
}

type orchestrationPreviewResponse struct {
	Status          string                      `json:"status"`
	GraphSnapshotID string                      `json:"graph_snapshot_id,omitempty"`
	Entrypoint      string                      `json:"entrypoint,omitempty"`
	RouteDecision   orchestration.RouteDecision `json:"route_decision"`
	Tasks           []orchestrationPreviewTask  `json:"tasks"`
	Warnings        []string                    `json:"warnings,omitempty"`
}

type orchestrationPreviewTask struct {
	AgentID         string         `json:"agent_id"`
	RuntimeID       string         `json:"runtime_id,omitempty"`
	RoleID          string         `json:"role_id,omitempty"`
	Sequence        int            `json:"sequence,omitempty"`
	Purpose         string         `json:"purpose,omitempty"`
	SelectionReason string         `json:"selection_reason,omitempty"`
	RequiredTools   []string       `json:"required_tools,omitempty"`
	RequiredSkills  []string       `json:"required_skills,omitempty"`
	OutputContract  map[string]any `json:"output_contract,omitempty"`
}

type timelineItem struct {
	ID       string          `json:"id"`
	Kind     string          `json:"kind"`
	Time     string          `json:"time"`
	Status   string          `json:"status,omitempty"`
	Title    string          `json:"title"`
	AgentID  string          `json:"agent_id,omitempty"`
	TaskID   string          `json:"task_id,omitempty"`
	ToolID   string          `json:"tool_id,omitempty"`
	Artifact string          `json:"artifact_id,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
	SortTime time.Time       `json:"-"`
	Sequence int             `json:"-"`
}

type todoItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	AgentID   string `json:"agent_id,omitempty"`
	TaskID    string `json:"task_id,omitempty"`
	Summary   string `json:"summary,omitempty"`
	BlockedBy string `json:"blocked_by,omitempty"`
}

type harnessEvalResponse struct {
	Status  string              `json:"status"`
	Passed  int                 `json:"passed"`
	Failed  int                 `json:"failed"`
	Cases   []harnessEvalResult `json:"cases"`
	Summary string              `json:"summary"`
}

type harnessEvalResult struct {
	Name      string                      `json:"name"`
	Prompt    string                      `json:"prompt"`
	Pass      bool                        `json:"pass"`
	Expected  string                      `json:"expected"`
	Decision  orchestration.RouteDecision `json:"decision"`
	Issue     string                      `json:"issue,omitempty"`
	ToolGaps  []string                    `json:"tool_gaps,omitempty"`
	SkillGaps []string                    `json:"skill_gaps,omitempty"`
}

func agentTemplatesHandler() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		writeSuccess(response, newRequestID(), agentTemplates(), nil)
	}
}

func agentTestHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		agentID := chi.URLParam(request, "agent_id")
		agent, err := projectconfig.GetAgent(options.ConfigPath, agentID)
		if err != nil {
			writeProjectConfigError(response, requestID, err)
			return
		}
		if err := projectconfig.ValidateAgent(*agent); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "agent_invalid", err.Error(), "Fix the agent fields and retry.")
			return
		}
		var body agentTestRequest
		if request.Body != nil {
			_ = json.NewDecoder(request.Body).Decode(&body)
		}
		execute := true
		if body.Execute != nil {
			execute = *body.Execute
		}
		if strings.TrimSpace(body.Prompt) == "" {
			body.Prompt = "Reply with one concise sentence confirming this agent is ready."
		}
		if !execute || agent.Kind == agentspec.AgentKindExternal {
			mode := "validation_only"
			warnings := []string{}
			if agent.Kind == agentspec.AgentKindExternal {
				warnings = append(warnings, "External agents are validated here; run them from Chat or CLI to exercise their command runtime.")
			}
			writeSuccess(response, requestID, agentTestResponse{AgentID: agentID, Status: "valid", Mode: mode, Warnings: warnings}, nil)
			return
		}
		if services.Graph == nil || services.Trace == nil || services.Secrets == nil || services.Adapter == nil {
			writeSuccess(response, requestID, agentTestResponse{AgentID: agentID, Status: "valid", Mode: "validation_only", Warnings: []string{"Runtime services are not available, so the test stopped after validation."}}, nil)
			return
		}
		snapshot, err := services.Graph.Latest(request.Context())
		if err != nil {
			writeSuccess(response, requestID, agentTestResponse{AgentID: agentID, Status: "valid", Mode: "validation_only", Warnings: []string{"No compiled graph snapshot is available. Save or validate the graph, then test execution."}}, nil)
			return
		}
		result, err := runExecutor(options, services).Execute(request.Context(), runs.Request{
			Snapshot: snapshot,
			AgentID:  agentID,
			Prompt:   body.Prompt,
		})
		if err != nil {
			writeError(response, http.StatusBadRequest, requestID, "agent_test_failed", err.Error(), "Check provider readiness, model profile, or runtime configuration.")
			return
		}
		writeSuccess(response, requestID, agentTestResponse{
			AgentID:   agentID,
			Status:    result.Status,
			Mode:      "executed",
			RunID:     result.RunID,
			Output:    agentTestOutput(result),
			TraceHint: "Open Runs to inspect the test trace.",
		}, nil)
	}
}

func orchestrationPreviewHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		var body orchestrationPreviewRequest
		if request.Body != nil {
			_ = json.NewDecoder(request.Body).Decode(&body)
		}
		preview, err := buildOrchestrationPreview(request.Context(), options, services, body)
		if err != nil {
			writeError(response, http.StatusBadRequest, requestID, "orchestration_preview_failed", err.Error(), "Fix the graph or orchestration settings and retry.")
			return
		}
		writeSuccess(response, requestID, preview, nil)
	}
}

func orchestrationTestHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		var body orchestrationPreviewRequest
		if request.Body != nil {
			_ = json.NewDecoder(request.Body).Decode(&body)
		}
		preview, err := buildOrchestrationPreview(request.Context(), options, services, body)
		if err != nil {
			writeError(response, http.StatusBadRequest, requestID, "orchestration_test_failed", err.Error(), "Fix the graph or orchestration settings and retry.")
			return
		}
		status := "valid"
		warnings := append([]string{}, preview.Warnings...)
		if preview.Entrypoint == "" {
			status = "invalid"
			warnings = append(warnings, "No entrypoint could be selected.")
		}
		if len(preview.Tasks) == 0 {
			status = "invalid"
			warnings = append(warnings, "No executable role tasks were produced.")
		}
		preview.Status = status
		preview.Warnings = warnings
		writeSuccess(response, requestID, preview, nil)
	}
}

func sessionTimelineHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		items, err := sessionTimeline(request.Context(), services, chi.URLParam(request, "session_id"))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
				writeError(response, http.StatusNotFound, requestID, "session_not_found", "Run session was not found.", "Refresh recent sessions.")
				return
			}
			writeError(response, http.StatusInternalServerError, requestID, "timeline_failed", "Session timeline could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, items, nil)
	}
}

func sessionTodosHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Runs == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "sessions_unavailable", "Run session store is not initialized.", "Restart Gateway.")
			return
		}
		detail, err := services.Runs.GetBySession(request.Context(), chi.URLParam(request, "session_id"))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
				writeError(response, http.StatusNotFound, requestID, "session_not_found", "Run session was not found.", "Refresh recent sessions.")
				return
			}
			writeError(response, http.StatusInternalServerError, requestID, "todos_failed", "Session todos could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, taskTodos(detail.Tasks), nil)
	}
}

func harnessEvalHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		snapshot, _ := latestSnapshot(request, services)
		writeSuccess(response, newRequestID(), runHarnessEval(snapshot), nil)
	}
}

func agentTemplates() []agentTemplate {
	return []agentTemplate{
		{
			ID:           "planner",
			Name:         "Planner",
			Description:  "Turns messy goals into bounded, reviewable execution plans.",
			Kind:         agentspec.AgentKindModel,
			Role:         "Plan long-horizon work, identify risks, and define handoff-ready next steps.",
			Instructions: "Keep plans short, specific, and testable. Prefer one active path over speculative branches.",
			Skills:       []string{"planning"},
			Tags:         []string{"planning", "coordination"},
			Triggers:     []string{"plan", "scope", "break down", "coordinate"},
		},
		{
			ID:           "researcher",
			Name:         "Researcher",
			Description:  "Collects project and web facts before implementation decisions.",
			Kind:         agentspec.AgentKindModel,
			Role:         "Research source material, cite evidence, and summarize tradeoffs.",
			Instructions: "Separate observed facts from inferences. Use read-only tools before recommending changes.",
			Tools:        []string{"read_project", "search", "fetch"},
			Skills:       []string{"research"},
			Tags:         []string{"research"},
			Triggers:     []string{"research", "compare", "investigate", "facts"},
		},
		{
			ID:             "coder",
			Name:           "Coder",
			Description:    "Implements and verifies controlled workspace changes.",
			Kind:           agentspec.AgentKindModel,
			Role:           "Make focused code changes and run verification through mediated tools.",
			Instructions:   "Use file and bash tools only through Nomici. Explain failures and leave artifacts for review.",
			Tools:          []string{"read_project", "write_project", "run_checks"},
			Skills:         []string{"coding"},
			Tags:           []string{"implementation"},
			Triggers:       []string{"implement", "fix", "test", "build"},
			ApprovalPolicy: "ask",
			Permissions:    map[string]any{"filesystem": "approval", "bash": "approval"},
		},
		{
			ID:           "reporter",
			Name:         "Reporter",
			Description:  "Turns completed work into final user-facing artifacts.",
			Kind:         agentspec.AgentKindModel,
			Role:         "Summarize outcomes, decisions, test results, and remaining risks.",
			Instructions: "Write concise final reports grounded in task summaries, tool output, and artifacts.",
			Skills:       []string{"reporting"},
			Tags:         []string{"artifact", "reporting"},
			Triggers:     []string{"summarize", "report", "deliver"},
		},
	}
}

func buildOrchestrationPreview(ctx context.Context, options Options, services Services, body orchestrationPreviewRequest) (*orchestrationPreviewResponse, error) {
	if services.Graph == nil {
		return nil, errors.New("graph store is not initialized")
	}
	snapshot, err := services.Graph.Latest(ctx)
	if err != nil {
		return nil, err
	}
	prompt := strings.TrimSpace(body.Prompt)
	if prompt == "" {
		prompt = "Preview a long-horizon workspace task."
	}
	agentID := strings.TrimSpace(body.AgentID)
	config, _ := projectconfig.GetOrchestration(options.ConfigPath)
	if agentID == "" && config.Entrypoint != "" {
		agentID = config.Entrypoint
	}
	decision := orchestration.Route(prompt, agentID, snapshot)
	applyOrchestrationConfig(&decision, config)
	if decision.RecommendedAgentID != "" {
		agentID = decision.RecommendedAgentID
	}
	if agentID == "" {
		agentID = defaultRunAgent(snapshot)
	}
	plans, err := ledgerTaskPlans(ctx, services, snapshot, agentID, &decision)
	if err != nil {
		return nil, err
	}
	return &orchestrationPreviewResponse{
		Status:          "preview",
		GraphSnapshotID: snapshot.SnapshotID,
		Entrypoint:      agentID,
		RouteDecision:   decision,
		Tasks:           previewTasks(plans),
		Warnings:        previewWarnings(snapshot, plans),
	}, nil
}

func previewTasks(plans []ledgerTaskPlan) []orchestrationPreviewTask {
	tasks := make([]orchestrationPreviewTask, 0, len(plans))
	for index, plan := range plans {
		task := orchestrationPreviewTask{
			AgentID:   plan.AgentID,
			RuntimeID: plan.RuntimeID,
			Sequence:  index + 1,
		}
		if roleID, ok := plan.Metadata["role_id"].(string); ok {
			task.RoleID = roleID
		}
		if sequence, ok := plan.Metadata["sequence"].(int); ok {
			task.Sequence = sequence
		}
		if purpose, ok := plan.Metadata["purpose"].(string); ok {
			task.Purpose = purpose
		}
		if reason, ok := plan.Metadata["selection_reason"].(string); ok {
			task.SelectionReason = reason
		}
		task.RequiredTools = stringListMetadata(plan.Metadata, "required_tools")
		task.RequiredSkills = stringListMetadata(plan.Metadata, "required_skills")
		if raw, ok := plan.Metadata["output_contract"]; ok {
			payload, _ := json.Marshal(raw)
			_ = json.Unmarshal(payload, &task.OutputContract)
		}
		tasks = append(tasks, task)
	}
	return tasks
}

func previewWarnings(snapshot *graph.Snapshot, plans []ledgerTaskPlan) []string {
	warnings := []string{}
	for _, plan := range plans {
		agent, ok := snapshot.IR.Agents[plan.AgentID]
		if !ok {
			warnings = append(warnings, "Agent "+plan.AgentID+" is not in the current graph.")
			continue
		}
		if (agent.Kind == agentspec.AgentKindModel || agent.Kind == agentspec.AgentKindGateway) && agent.Model == "" {
			warnings = append(warnings, "Agent "+plan.AgentID+" has no model profile.")
		}
		if agent.Kind == agentspec.AgentKindExternal && agent.Runtime == "" {
			warnings = append(warnings, "Agent "+plan.AgentID+" has no runtime.")
		}
	}
	return warnings
}

func stringListMetadata(metadata map[string]any, key string) []string {
	raw, ok := metadata[key]
	if !ok {
		return nil
	}
	switch typed := raw.(type) {
	case []string:
		return typed
	case []any:
		result := []string{}
		for _, value := range typed {
			text, ok := value.(string)
			if ok && text != "" {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

func agentTestOutput(result *runs.Result) string {
	if result == nil {
		return ""
	}
	if len(result.Messages) > 0 {
		return result.Messages[len(result.Messages)-1].Content
	}
	if result.CLI != nil {
		if strings.TrimSpace(result.CLI.Stdout) != "" {
			return result.CLI.Stdout
		}
		return result.CLI.Stderr
	}
	return ""
}

func sessionTimeline(ctx context.Context, services Services, sessionID string) ([]timelineItem, error) {
	if services.Runs == nil || services.Trace == nil {
		return nil, errors.New("run session services are not initialized")
	}
	detail, err := services.Runs.GetBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	items := []timelineItem{{
		ID:       detail.Session.SessionID,
		Kind:     "session",
		Time:     detail.Session.StartedAt.Format(time.RFC3339),
		SortTime: detail.Session.StartedAt,
		Status:   detail.Session.Status,
		Title:    detail.Session.Title,
	}}
	for _, task := range detail.Tasks {
		taskTime := task.StartedAt
		if taskTime.IsZero() {
			taskTime = task.UpdatedAt
		}
		items = append(items, timelineItem{
			ID:       task.TaskID,
			Kind:     "task",
			Time:     taskTime.Format(time.RFC3339),
			SortTime: taskTime,
			Status:   task.Status,
			Title:    taskTimelineTitle(task),
			AgentID:  task.AgentID,
			TaskID:   task.TaskID,
			Payload:  task.Metadata,
		})
	}
	events, err := services.Trace.ListByRun(ctx, detail.Session.RunID)
	if err != nil {
		return nil, err
	}
	for _, event := range events {
		items = append(items, timelineItem{
			ID:       event.EventID,
			Kind:     "trace",
			Time:     event.Time.Format(time.RFC3339),
			SortTime: event.Time,
			Sequence: event.Sequence,
			Title:    event.Type,
			AgentID:  event.NodeID,
			Payload:  event.Payload,
		})
	}
	if services.Tools != nil {
		records, err := services.Tools.ListBySession(ctx, sessionID, 100)
		if err != nil {
			return nil, err
		}
		for _, record := range records {
			items = append(items, timelineItem{
				ID:       record.ToolCallID,
				Kind:     "tool_call",
				Time:     record.UpdatedAt.Format(time.RFC3339),
				SortTime: record.UpdatedAt,
				Status:   record.Status,
				Title:    record.ToolID,
				TaskID:   record.TaskID,
				ToolID:   record.ToolID,
			})
		}
	}
	if services.Artifacts != nil {
		records, err := services.Artifacts.List(ctx, sessionID, 100)
		if err != nil {
			return nil, err
		}
		for _, artifact := range records {
			items = append(items, timelineItem{
				ID:       artifact.ArtifactID,
				Kind:     "artifact",
				Time:     artifact.UpdatedAt.Format(time.RFC3339),
				SortTime: artifact.UpdatedAt,
				Status:   artifact.ReviewState,
				Title:    artifact.Title,
				TaskID:   artifact.TaskID,
				Artifact: artifact.ArtifactID,
			})
		}
	}
	if services.Blocked != nil {
		actions, err := services.Blocked.ListBySession(ctx, sessionID, "", 100)
		if err != nil {
			return nil, err
		}
		for _, action := range actions {
			items = append(items, timelineItem{
				ID:       action.BlockedActionID,
				Kind:     "blocked_action",
				Time:     action.UpdatedAt.Format(time.RFC3339),
				SortTime: action.UpdatedAt,
				Status:   action.Status,
				Title:    action.Title,
				TaskID:   action.TaskID,
				Payload:  action.Metadata,
			})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SortTime.Equal(items[j].SortTime) {
			return items[i].Sequence < items[j].Sequence
		}
		return items[i].SortTime.Before(items[j].SortTime)
	})
	return items, nil
}

func taskTodos(tasks []*runs.Task) []todoItem {
	todos := make([]todoItem, 0, len(tasks))
	for _, task := range tasks {
		todos = append(todos, todoItem{
			ID:        task.TaskID,
			Title:     taskTimelineTitle(task),
			Status:    task.Status,
			AgentID:   task.AgentID,
			TaskID:    task.TaskID,
			Summary:   taskMetadataString(task, "summary"),
			BlockedBy: task.BlockedReason,
		})
	}
	return todos
}

func taskTimelineTitle(task *runs.Task) string {
	if task == nil {
		return "Task"
	}
	if purpose := taskMetadataString(task, "purpose"); purpose != "" {
		return purpose
	}
	return task.AgentID
}

func runHarnessEval(snapshot *graph.Snapshot) harnessEvalResponse {
	cases := []struct {
		Name      string
		Prompt    string
		WantMode  string
		WantTools []string
		WantRisk  string
	}{
		{Name: "basic-chat", Prompt: "hey", WantMode: orchestration.ModeDirectReply},
		{Name: "empty-clarify", Prompt: "", WantMode: orchestration.ModeClarify},
		{Name: "setup-question", Prompt: "how do I start the local console?", WantMode: orchestration.ModeDirectReply},
		{Name: "research-work", Prompt: "research this project architecture and summarize tradeoffs", WantMode: orchestration.ModeWorkspaceRun, WantTools: []string{"read_project"}},
		{Name: "code-mutation", Prompt: "fix the bug, edit files, and run tests", WantMode: orchestration.ModeWorkspaceRun, WantTools: []string{"write_project", "run_checks"}, WantRisk: "high"},
	}
	results := make([]harnessEvalResult, 0, len(cases))
	passed := 0
	for _, testCase := range cases {
		decision := orchestration.Route(testCase.Prompt, "", snapshot)
		pass := decision.Mode == testCase.WantMode
		issue := ""
		for _, tool := range testCase.WantTools {
			if !containsString(decision.RequiredTools, tool) {
				pass = false
				issue = "missing expected tool " + tool
			}
		}
		if testCase.WantRisk != "" && decision.Risk != testCase.WantRisk {
			pass = false
			issue = "expected risk " + testCase.WantRisk
		}
		if !pass && issue == "" {
			issue = "expected mode " + testCase.WantMode
		}
		if pass {
			passed++
		}
		results = append(results, harnessEvalResult{
			Name:     testCase.Name,
			Prompt:   testCase.Prompt,
			Pass:     pass,
			Expected: testCase.WantMode,
			Decision: decision,
			Issue:    issue,
		})
	}
	failed := len(cases) - passed
	status := "passed"
	if failed > 0 {
		status = "failed"
	}
	return harnessEvalResponse{
		Status:  status,
		Passed:  passed,
		Failed:  failed,
		Cases:   results,
		Summary: "Harness eval checks chat routing, clarification, workspace routing, tool need detection, and high-risk mutation handling.",
	}
}
