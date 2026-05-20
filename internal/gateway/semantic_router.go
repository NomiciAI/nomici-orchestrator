package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/adapters"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/orchestration"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
)

func routeChatIntent(ctx context.Context, services Services, prompt string, manualAgentID string, snapshot *graph.Snapshot) orchestration.RouteDecision {
	fallback := orchestration.Route(prompt, manualAgentID, snapshot)
	semantic, err := semanticRoute(ctx, services, prompt, manualAgentID, snapshot, fallback)
	if err != nil {
		return fallback
	}
	mergeRouteDefaults(&semantic, fallback)
	return semantic
}

func semanticRoute(ctx context.Context, services Services, prompt string, manualAgentID string, snapshot *graph.Snapshot, fallback orchestration.RouteDecision) (orchestration.RouteDecision, error) {
	if services.Adapter == nil || services.Secrets == nil || snapshot == nil {
		return orchestration.RouteDecision{}, fmt.Errorf("semantic router unavailable")
	}
	agentID := fallback.RecommendedAgentID
	if manualAgentID != "" {
		agentID = manualAgentID
	}
	agent, ok := snapshot.IR.Agents[agentID]
	if !ok || agent.Model == "" {
		return orchestration.RouteDecision{}, fmt.Errorf("semantic router agent unavailable")
	}
	model, ok := snapshot.IR.Models[agent.Model]
	if !ok {
		return orchestration.RouteDecision{}, fmt.Errorf("semantic router model unavailable")
	}
	profile := graphModelToGatewayProvider(model)
	if model.Profile != "" && services.Providers != nil {
		stored, err := services.Providers.Get(ctx, model.Profile)
		if err != nil && err != sql.ErrNoRows {
			return orchestration.RouteDecision{}, err
		}
		if stored != nil {
			profile = stored
		}
	}
	var apiKey string
	if profile.APIKeyEnv != "" {
		resolved, ok := services.Secrets.ResolveEnv(profile.APIKeyEnv)
		if !ok {
			return orchestration.RouteDecision{}, fmt.Errorf("semantic router missing provider secret")
		}
		apiKey = resolved
	}
	routerPrompt := semanticRouterPrompt(prompt, manualAgentID, snapshot, fallback)
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	result, err := services.Adapter.Invoke(ctx, adapters.ModelConfig{
		Kind:    profile.Kind,
		BaseURL: profile.BaseURL,
		Model:   profile.Model,
	}, apiKey, adapters.InvokeRequest{
		RunID:  "",
		NodeID: "chat_router",
		Messages: []adapters.Message{
			{Role: "system", Content: "Return only valid JSON. Do not wrap it in markdown."},
			{Role: "user", Content: routerPrompt},
		},
		Options: adapters.InvokeOptions{TimeoutMs: 45000},
	})
	if err != nil {
		return orchestration.RouteDecision{}, err
	}
	if result == nil || result.Status != adapters.StatusCompleted || len(result.Messages) == 0 {
		return orchestration.RouteDecision{}, fmt.Errorf("semantic router did not complete")
	}
	decision, err := parseRouteDecision(result.Messages[len(result.Messages)-1].Content)
	if err != nil {
		repairPrompt := "Repair this response into valid route JSON matching the schema. Return JSON only.\n\n" + result.Messages[len(result.Messages)-1].Content
		repaired, repairErr := services.Adapter.Invoke(ctx, adapters.ModelConfig{
			Kind:    profile.Kind,
			BaseURL: profile.BaseURL,
			Model:   profile.Model,
		}, apiKey, adapters.InvokeRequest{
			NodeID:   "chat_router_repair",
			Messages: []adapters.Message{{Role: "user", Content: repairPrompt}},
			Options:  adapters.InvokeOptions{TimeoutMs: 30000},
		})
		if repairErr != nil || repaired == nil || repaired.Status != adapters.StatusCompleted || len(repaired.Messages) == 0 {
			return orchestration.RouteDecision{}, err
		}
		decision, err = parseRouteDecision(repaired.Messages[len(repaired.Messages)-1].Content)
		if err != nil {
			return orchestration.RouteDecision{}, err
		}
	}
	if !validRouteMode(decision.Mode) {
		return orchestration.RouteDecision{}, fmt.Errorf("semantic router returned unsupported mode")
	}
	return decision, nil
}

func semanticRouterPrompt(prompt string, manualAgentID string, snapshot *graph.Snapshot, fallback orchestration.RouteDecision) string {
	agents := []string{}
	if snapshot != nil {
		for id, agent := range snapshot.IR.Agents {
			parts := []string{id, agent.Kind}
			if agent.Description != "" {
				parts = append(parts, agent.Description)
			}
			if len(agent.Tools) > 0 {
				parts = append(parts, "tools="+strings.Join(agent.Tools, ","))
			}
			if len(agent.Skills) > 0 {
				parts = append(parts, "skills="+strings.Join(agent.Skills, ","))
			}
			agents = append(agents, strings.Join(parts, " | "))
		}
	}
	return fmt.Sprintf(`Classify the chat message for a local long-horizon agent workspace.

Schema:
{
  "mode": "direct_reply | clarify | workspace_run",
  "goal": "normalized user goal",
  "complexity": "simple | medium | long_horizon",
  "missing_inputs": ["short missing input names"],
  "recommended_agent_id": "agent id or empty",
  "selected_roles": ["role ids"],
  "required_tools": ["read_project", "write_project", "run_checks", "search", "fetch"],
  "required_skills": ["planning", "research", "coding", "reporting"],
  "needs_plan_review": true,
  "risk": "low | medium | high | critical",
  "confidence": 0.0,
  "rationale": "one concise sentence",
  "clarification": "question when mode is clarify"
}

Rules:
- Prefer workspace_run when the request may require planning, files, commands, research, or artifacts.
- Use direct_reply only for setup/status/navigation/help that does not need a run.
- Use clarify when required inputs are missing and executing would be unsafe.
- Set needs_plan_review for mutation, shell, file writes, commits, deploys, or irreversible work.
- If a manual agent was provided, keep it as recommended_agent_id.

Manual agent: %s
Fallback route: %+v
Available agents:
%s

Chat message:
%s`, manualAgentID, fallback, strings.Join(agents, "\n"), prompt)
}

func parseRouteDecision(content string) (orchestration.RouteDecision, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	var decision orchestration.RouteDecision
	if err := json.Unmarshal([]byte(content), &decision); err != nil {
		return orchestration.RouteDecision{}, err
	}
	return decision, nil
}

func mergeRouteDefaults(decision *orchestration.RouteDecision, fallback orchestration.RouteDecision) {
	if strings.TrimSpace(decision.Goal) == "" {
		decision.Goal = fallback.Goal
	}
	if strings.TrimSpace(decision.RecommendedAgentID) == "" {
		decision.RecommendedAgentID = fallback.RecommendedAgentID
	}
	if strings.TrimSpace(decision.Complexity) == "" {
		decision.Complexity = fallback.Complexity
	}
	if decision.Confidence == 0 {
		decision.Confidence = fallback.Confidence
	}
	if strings.TrimSpace(decision.Risk) == "" {
		decision.Risk = fallback.Risk
	}
	if strings.TrimSpace(decision.Rationale) == "" {
		decision.Rationale = fallback.Rationale
	}
	decision.ManualAgentID = fallback.ManualAgentID
	decision.RequiredTools = uniqueStrings(append(decision.RequiredTools, fallback.RequiredTools...))
	decision.RequiredSkills = uniqueStrings(append(decision.RequiredSkills, fallback.RequiredSkills...))
	if fallback.NeedsPlanReview {
		decision.NeedsPlanReview = true
	}
}

func validRouteMode(mode string) bool {
	switch mode {
	case orchestration.ModeDirectReply, orchestration.ModeClarify, orchestration.ModeWorkspaceRun:
		return true
	default:
		return false
	}
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func graphModelToGatewayProvider(model graph.Model) *providers.Profile {
	capabilities := map[string]string{}
	for _, capability := range model.Capabilities {
		capabilities[capability] = "true"
	}
	return &providers.Profile{
		ID:            model.ID,
		Name:          model.ID,
		Kind:          model.Kind,
		BaseURL:       model.BaseURL,
		Model:         model.Model,
		APIKeyEnv:     model.APIKeyEnv,
		Capabilities:  capabilities,
		ContextWindow: model.ContextWindow,
	}
}
