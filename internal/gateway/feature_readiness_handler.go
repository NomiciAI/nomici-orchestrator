package gateway

import (
	"net/http"
)

type featureReadinessItem struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

func featureReadinessHandler(options Options, services Services) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		writeSuccess(response, newRequestID(), featureReadiness(request, options, services), nil)
	}
}

func featureReadiness(request *http.Request, options Options, services Services) []featureReadinessItem {
	hasModel := false
	if services.Providers != nil {
		if profiles, err := services.Providers.List(request.Context()); err == nil && len(profiles) > 0 {
			hasModel = true
		}
	}
	graphReady := false
	if services.Graph != nil {
		if snapshot, err := services.Graph.Latest(request.Context()); err == nil && snapshotHasRunnableAgents(snapshot) {
			graphReady = true
		} else if hasModel {
			graphReady = true
		}
	}
	toolsReady := services.Tools != nil && services.Policy != nil && services.Runs != nil && services.Sandboxes != nil
	items := []featureReadinessItem{
		readiness("chat", "Chat", hasModel, "A configured model profile is required before chat can answer honestly."),
		readiness("default_team", "Built-in default team", hasModel, "A model profile is required to materialize the default team."),
		readiness("agents", "Agent Studio", services.Graph != nil && hasModel, "Agent create/update is available; execution tests require a model and graph store."),
		readiness("orchestration_preview", "Flow Preview", services.Graph != nil, "Preview is diagnostic and does not execute."),
		readiness("orchestration_test_run", "Orchestration Test Run", graphReady, "A runnable graph or materialized default team is required."),
		readiness("tool_loop", "Tool-backed runs", toolsReady, "Tool execution needs stores, policy, approvals, sandbox, and run ledger services."),
		readiness("artifacts", "Artifact Center", services.Artifacts != nil, "Artifact storage is not initialized."),
		readiness("memory", "Memory review", services.Memory != nil && services.Context != nil, "Memory proposals require memory and shared context stores."),
	}
	for _, definition := range toolDefinitionsWithStatus(options, services) {
		status := "works"
		if definition.ExecutionStatus == "configured_only" {
			status = "diagnostic"
		}
		if definition.ExecutionStatus == "unavailable" {
			status = "hidden"
		}
		items = append(items, featureReadinessItem{
			ID:     "tool." + definition.ID,
			Label:  "Tool: " + definition.ID,
			Status: status,
			Reason: definition.ExecutionStatus,
		})
	}
	return items
}

func readiness(id string, label string, ready bool, reason string) featureReadinessItem {
	if ready {
		return featureReadinessItem{ID: id, Label: label, Status: "works"}
	}
	return featureReadinessItem{ID: id, Label: label, Status: "diagnostic", Reason: reason}
}
