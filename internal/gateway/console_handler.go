package gateway

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sort"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/packs"
	"github.com/NomiciAI/nomici-orchestrator/internal/policy"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	runpkg "github.com/NomiciAI/nomici-orchestrator/internal/runs"
	"github.com/NomiciAI/nomici-orchestrator/internal/sharedcontext"
	"github.com/NomiciAI/nomici-orchestrator/internal/trace"
)

const consoleRunLimit = 8

type consoleOverview struct {
	Gateway          consoleGatewayStatus    `json:"gateway"`
	Counts           consoleCounts           `json:"counts"`
	Models           []consoleModelProfile   `json:"models"`
	Tools            []consoleToolStatus     `json:"tools"`
	Packs            []consolePackStatus     `json:"packs"`
	Graph            *consoleGraphSummary    `json:"graph,omitempty"`
	GraphSnapshot    *graph.Snapshot         `json:"graph_snapshot,omitempty"`
	Runtimes         []consoleRuntimeStatus  `json:"runtimes"`
	RecentRuns       []*trace.RunSummary     `json:"recent_runs"`
	RecentSessions   []*runpkg.Session       `json:"recent_sessions"`
	LatestTrace      []consoleTraceEvent     `json:"latest_trace"`
	PendingApprovals []*policy.Approval      `json:"pending_approvals"`
	Unavailable      []consoleUnavailableAPI `json:"unavailable"`
}

type consoleGatewayStatus struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}

type consoleCounts struct {
	Models           int `json:"models"`
	Tools            int `json:"tools"`
	PacksInstalled   int `json:"packs_installed"`
	Agents           int `json:"agents"`
	Runtimes         int `json:"runtimes"`
	Runs             int `json:"runs"`
	PendingApprovals int `json:"pending_approvals"`
}

type consoleModelProfile struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Kind            string            `json:"kind"`
	BaseURL         string            `json:"base_url"`
	Model           string            `json:"model"`
	APIKeyEnv       string            `json:"api_key_env"`
	Capabilities    map[string]string `json:"capabilities,omitempty"`
	ContextWindow   int               `json:"context_window,omitempty"`
	CostPer1MInput  float64           `json:"cost_per_1m_input,omitempty"`
	CostPer1MOutput float64           `json:"cost_per_1m_output,omitempty"`
	CreatedAt       string            `json:"created_at"`
	UpdatedAt       string            `json:"updated_at"`
}

type consoleToolStatus struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Provider  string `json:"provider"`
	Mode      string `json:"mode"`
	Status    string `json:"status"`
	Auth      string `json:"auth"`
	Execution string `json:"execution"`
}

type consolePackStatus struct {
	Manifest     packs.Manifest      `json:"manifest"`
	Installed    bool                `json:"installed"`
	Installation *packs.Installation `json:"installation,omitempty"`
}

type consoleGraphSummary struct {
	SnapshotID    string `json:"snapshot_id"`
	ProjectID     string `json:"project_id"`
	SchemaVersion string `json:"schema_version"`
	SourceHash    string `json:"source_hash"`
	CreatedAt     string `json:"created_at"`
	ModelCount    int    `json:"model_count"`
	AgentCount    int    `json:"agent_count"`
	RuntimeCount  int    `json:"runtime_count"`
	EdgeCount     int    `json:"edge_count"`
}

type consoleRuntimeStatus struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	Runner    string   `json:"runner,omitempty"`
	Workspace string   `json:"workspace,omitempty"`
	Trust     string   `json:"trust,omitempty"`
	Status    string   `json:"status"`
	Agents    []string `json:"agents,omitempty"`
}

type consoleTraceEvent struct {
	EventID    string          `json:"event_id"`
	RunID      string          `json:"run_id"`
	Sequence   int             `json:"sequence"`
	Type       string          `json:"type"`
	Time       string          `json:"time"`
	NodeID     string          `json:"node_id,omitempty"`
	RuntimeID  string          `json:"runtime_id,omitempty"`
	Payload    json.RawMessage `json:"payload"`
	Redactions []string        `json:"redactions"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
}

type consoleUnavailableAPI struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func consoleOverviewHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		overview, warnings, err := buildConsoleOverview(request, options, services)
		if err != nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "console_state_unavailable", err.Error(), "Restart Gateway or check database initialization.")
			return
		}
		writeSuccess(response, requestID, overview, warnings)
	}
}

func modelListHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Providers == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "models_unavailable", "Model registry is not initialized.", "Restart Gateway.")
			return
		}
		models, err := services.Providers.List(request.Context())
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "models_list_failed", "Model profiles could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, sanitizeModelProfiles(models), nil)
	}
}

func packListHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		packStatuses, err := loadPackStatuses(request, services)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "packs_list_failed", "Pack status could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, packStatuses, nil)
	}
}

func latestGraphHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		snapshot, err := latestSnapshot(request, services)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(response, http.StatusNotFound, requestID, "graph_not_found", "No compiled graph snapshot was found.", "Run `nomici graph validate`.")
				return
			}
			writeError(response, http.StatusInternalServerError, requestID, "graph_load_failed", "Latest graph snapshot could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, snapshot, nil)
	}
}

func runtimeListHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		snapshot, err := latestSnapshot(request, services)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeSuccess(response, requestID, []consoleRuntimeStatus{}, []string{"No compiled graph snapshot yet; run `nomici graph validate`."})
				return
			}
			writeError(response, http.StatusInternalServerError, requestID, "runtimes_load_failed", "Runtime state could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, runtimeStatuses(snapshot), nil)
	}
}

func runListHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Trace == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "runs_unavailable", "Trace store is not initialized.", "Restart Gateway.")
			return
		}
		runs, err := services.Trace.ListRuns(request.Context())
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "runs_list_failed", "Recent runs could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, limitRuns(runs), nil)
	}
}

func approvalListHandler(services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if services.Policy == nil {
			writeError(response, http.StatusServiceUnavailable, requestID, "approvals_unavailable", "Policy service is not initialized.", "Restart Gateway.")
			return
		}
		status := request.URL.Query().Get("status")
		approvals, err := services.Policy.List(request.Context(), status)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "approvals_list_failed", "Approvals could not be loaded.", "Check Gateway logs.")
			return
		}
		writeSuccess(response, requestID, approvals, nil)
	}
}

func buildConsoleOverview(request *http.Request, options Options, services Services) (*consoleOverview, []string, error) {
	if services.Providers == nil || services.Trace == nil || services.Packs == nil || services.Policy == nil {
		return nil, nil, errors.New("one or more Console services are not initialized")
	}

	models, err := services.Providers.List(request.Context())
	if err != nil {
		return nil, nil, err
	}
	packStatuses, err := loadPackStatuses(request, services)
	if err != nil {
		return nil, nil, err
	}
	tools, toolWarnings, err := loadConsoleTools(options.ConfigPath)
	if err != nil {
		return nil, nil, err
	}
	runs, err := services.Trace.ListRuns(request.Context())
	if err != nil {
		return nil, nil, err
	}
	var sessions []*runpkg.Session
	if services.Runs != nil {
		sessions, err = services.Runs.ListSessions(request.Context(), consoleRunLimit)
		if err != nil {
			return nil, nil, err
		}
	}
	pendingApprovals, err := services.Policy.List(request.Context(), policy.StatusPending)
	if err != nil {
		return nil, nil, err
	}
	latestTrace, err := loadLatestTrace(request, services, runs)
	if err != nil {
		return nil, nil, err
	}

	var warnings []string
	warnings = append(warnings, toolWarnings...)
	var graphSummary *consoleGraphSummary
	var runtimes []consoleRuntimeStatus
	var agentCount int
	snapshot, err := latestSnapshot(request, services)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			warnings = append(warnings, "No compiled graph snapshot yet; run `nomici graph validate`.")
		} else {
			return nil, nil, err
		}
	} else {
		graphSummary = summarizeGraph(snapshot)
		runtimes = runtimeStatuses(snapshot)
		agentCount = len(snapshot.IR.Agents)
	}

	installedPacks := 0
	for _, status := range packStatuses {
		if status.Installed {
			installedPacks++
		}
	}

	version := options.Version
	if version == "" {
		version = "dev"
	}
	return &consoleOverview{
		Gateway: consoleGatewayStatus{
			Status:  "ok",
			Service: "nomici-gateway",
			Version: version,
		},
		Counts: consoleCounts{
			Models:           len(models),
			Tools:            len(tools),
			PacksInstalled:   installedPacks,
			Agents:           agentCount,
			Runtimes:         len(runtimes),
			Runs:             len(runs),
			PendingApprovals: len(pendingApprovals),
		},
		Models:           sanitizeModelProfiles(models),
		Tools:            tools,
		Packs:            packStatuses,
		Graph:            graphSummary,
		GraphSnapshot:    snapshot,
		Runtimes:         runtimes,
		RecentRuns:       limitRuns(runs),
		RecentSessions:   sessions,
		LatestTrace:      latestTrace,
		PendingApprovals: pendingApprovals,
		Unavailable: []consoleUnavailableAPI{
			{Name: "Canvas editing", Status: "deferred", Reason: "Gate 8 is read-only."},
			{Name: "Console provider setup", Status: "deferred", Reason: "Use `nomici setup` for bootstrap."},
			{Name: "Mediated tool execution", Status: "deferred", Reason: "Web tools are configured as read-only contracts."},
			{Name: "Runtime lifecycle controls", Status: "deferred", Reason: "Runtime reconciler is not implemented yet."},
		},
	}, warnings, nil
}

func loadPackStatuses(request *http.Request, services Services) ([]consolePackStatus, error) {
	installations := map[string]*packs.Installation{}
	if services.Packs != nil {
		records, err := services.Packs.ListInstallations(request.Context())
		if err != nil {
			return nil, err
		}
		for _, installation := range records {
			installations[installation.PackID] = installation
		}
	}

	statuses := make([]consolePackStatus, 0, len(packs.ListBuiltins()))
	for _, manifest := range packs.ListBuiltins() {
		installation := installations[manifest.ID]
		statuses = append(statuses, consolePackStatus{
			Manifest:     manifest,
			Installed:    installation != nil,
			Installation: installation,
		})
	}
	return statuses, nil
}

func loadConsoleTools(configPath string) ([]consoleToolStatus, []string, error) {
	if configPath == "" {
		configPath = "nomici.yaml"
	}
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return []consoleToolStatus{}, []string{"No AgentSpec config found; run `nomici setup`."}, nil
		}
		return nil, nil, err
	}
	loaded, err := agentspec.LoadFile(configPath)
	if err != nil {
		return nil, nil, err
	}
	ids := make([]string, 0, len(loaded.Spec.Tools))
	for id := range loaded.Spec.Tools {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	tools := make([]consoleToolStatus, 0, len(ids))
	for _, id := range ids {
		config := loaded.Spec.Tools[id]
		tools = append(tools, consoleToolStatus{
			ID:        id,
			Kind:      toolString(config, "kind", id),
			Provider:  toolString(config, "provider", "unknown"),
			Mode:      toolString(config, "mode", "unknown"),
			Status:    toolString(config, "status", "configured"),
			Auth:      toolString(config, "auth", "none"),
			Execution: toolString(config, "execution", "configured_not_executed"),
		})
	}
	return tools, nil, nil
}

func toolString(config map[string]any, key string, fallback string) string {
	value, _ := config[key].(string)
	if value == "" {
		return fallback
	}
	return value
}

func sanitizeModelProfiles(models []*providers.Profile) []consoleModelProfile {
	sanitized := make([]consoleModelProfile, 0, len(models))
	for _, model := range models {
		sanitized = append(sanitized, consoleModelProfile{
			ID:              model.ID,
			Name:            model.Name,
			Kind:            model.Kind,
			BaseURL:         model.BaseURL,
			Model:           model.Model,
			APIKeyEnv:       sharedcontext.RedactText(model.APIKeyEnv),
			Capabilities:    model.Capabilities,
			ContextWindow:   model.ContextWindow,
			CostPer1MInput:  model.CostPer1MInput,
			CostPer1MOutput: model.CostPer1MOutput,
			CreatedAt:       model.CreatedAt,
			UpdatedAt:       model.UpdatedAt,
		})
	}
	return sanitized
}

func loadLatestTrace(request *http.Request, services Services, runs []*trace.RunSummary) ([]consoleTraceEvent, error) {
	if len(runs) == 0 {
		return []consoleTraceEvent{}, nil
	}
	events, err := services.Trace.ListByRun(request.Context(), runs[0].RunID)
	if err != nil {
		return nil, err
	}
	timeline := make([]consoleTraceEvent, 0, len(events))
	for _, event := range events {
		timeline = append(timeline, consoleTraceEvent{
			EventID:    event.EventID,
			RunID:      event.RunID,
			Sequence:   event.Sequence,
			Type:       event.Type,
			Time:       event.Time.Format("2006-01-02T15:04:05Z07:00"),
			NodeID:     event.NodeID,
			RuntimeID:  event.RuntimeID,
			Payload:    event.Payload,
			Redactions: event.Redactions,
			Metadata:   event.Metadata,
		})
	}
	return timeline, nil
}

func latestSnapshot(request *http.Request, services Services) (*graph.Snapshot, error) {
	if services.Graph == nil {
		return nil, errors.New("graph store is not initialized")
	}
	return services.Graph.Latest(request.Context())
}

func summarizeGraph(snapshot *graph.Snapshot) *consoleGraphSummary {
	return &consoleGraphSummary{
		SnapshotID:    snapshot.SnapshotID,
		ProjectID:     snapshot.ProjectID,
		SchemaVersion: snapshot.SchemaVersion,
		SourceHash:    snapshot.SourceHash,
		CreatedAt:     snapshot.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		ModelCount:    len(snapshot.IR.Models),
		AgentCount:    len(snapshot.IR.Agents),
		RuntimeCount:  len(snapshot.IR.Runtimes),
		EdgeCount:     len(snapshot.IR.Edges),
	}
}

func runtimeStatuses(snapshot *graph.Snapshot) []consoleRuntimeStatus {
	agentsByRuntime := map[string][]string{}
	for _, agent := range snapshot.IR.Agents {
		if agent.Runtime != "" {
			agentsByRuntime[agent.Runtime] = append(agentsByRuntime[agent.Runtime], agent.ID)
		}
	}
	for runtimeID := range agentsByRuntime {
		sort.Strings(agentsByRuntime[runtimeID])
	}

	ids := make([]string, 0, len(snapshot.IR.Runtimes))
	for id := range snapshot.IR.Runtimes {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	statuses := make([]consoleRuntimeStatus, 0, len(ids))
	for _, id := range ids {
		runtime := snapshot.IR.Runtimes[id]
		statuses = append(statuses, consoleRuntimeStatus{
			ID:        runtime.ID,
			Kind:      runtime.Kind,
			Runner:    runtime.Runner,
			Workspace: runtime.Workspace,
			Trust:     runtime.Trust,
			Status:    "configured",
			Agents:    agentsByRuntime[id],
		})
	}
	return statuses
}

func limitRuns(runs []*trace.RunSummary) []*trace.RunSummary {
	if len(runs) <= consoleRunLimit {
		return runs
	}
	return runs[:consoleRunLimit]
}
