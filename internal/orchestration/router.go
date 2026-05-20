package orchestration

import (
	"sort"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/packs"
)

const (
	ModeWorkspaceRun = "workspace_run"
	ModeClarify      = "clarify"
	ModeDirectReply  = "direct_reply"

	ComplexitySimple      = "simple"
	ComplexityMedium      = "medium"
	ComplexityLongHorizon = "long_horizon"
)

type RouteDecision struct {
	Mode               string   `json:"mode"`
	Goal               string   `json:"goal"`
	Complexity         string   `json:"complexity"`
	RecommendedAgentID string   `json:"recommended_agent_id"`
	SelectedRoles      []string `json:"selected_roles"`
	NeedsPlanReview    bool     `json:"needs_plan_review"`
	RequiredTools      []string `json:"required_tools"`
	RequiredSkills     []string `json:"required_skills,omitempty"`
	MissingInputs      []string `json:"missing_inputs,omitempty"`
	Risk               string   `json:"risk,omitempty"`
	Confidence         float64  `json:"confidence,omitempty"`
	Rationale          string   `json:"rationale"`
	Clarification      string   `json:"clarification,omitempty"`
	ManualAgentID      string   `json:"manual_agent_id,omitempty"`
}

type RoleSelection struct {
	AgentID         string               `json:"agent_id"`
	RoleID          string               `json:"role_id"`
	Sequence        int                  `json:"sequence"`
	Purpose         string               `json:"purpose,omitempty"`
	SelectionReason string               `json:"selection_reason"`
	MatchScore      float64              `json:"match_score"`
	RequiredTools   []string             `json:"required_tools,omitempty"`
	RequiredSkills  []string             `json:"required_skills,omitempty"`
	OutputContract  packs.OutputContract `json:"output_contract,omitempty"`
}

type SkippedRole struct {
	RoleID string `json:"role_id"`
	Reason string `json:"reason"`
}

type MatchResult struct {
	Entrypoint      string          `json:"entrypoint"`
	Roles           []RoleSelection `json:"roles"`
	SkippedRoles    []SkippedRole   `json:"skipped_roles,omitempty"`
	RequiredTools   []string        `json:"required_tools,omitempty"`
	NeedsReview     bool            `json:"needs_plan_review"`
	SelectionReason string          `json:"selection_reason"`
}

func Route(prompt string, manualAgentID string, snapshot *graph.Snapshot) RouteDecision {
	goal := strings.Join(strings.Fields(prompt), " ")
	decision := RouteDecision{
		Mode:               ModeWorkspaceRun,
		Goal:               goal,
		Complexity:         ComplexityMedium,
		RecommendedAgentID: defaultAgent(snapshot),
		ManualAgentID:      strings.TrimSpace(manualAgentID),
		Confidence:         0.55,
		Risk:               "medium",
		Rationale:          "Defaulting to a workspace run so Nomici can plan, execute, and preserve artifacts.",
	}
	if decision.ManualAgentID != "" {
		decision.RecommendedAgentID = decision.ManualAgentID
		decision.Rationale = "Manual agent override was provided."
	}
	lower := strings.ToLower(goal)
	switch {
	case strings.TrimSpace(goal) == "":
		decision.Mode = ModeClarify
		decision.Complexity = ComplexitySimple
		decision.Clarification = "What outcome should Nomici deliver?"
		decision.MissingInputs = []string{"goal"}
		decision.Confidence = 0.95
		decision.Risk = "low"
		decision.Rationale = "The chat message was empty."
		return decision
	case isDirectQuery(lower):
		decision.Mode = ModeDirectReply
		decision.Complexity = ComplexitySimple
		decision.Confidence = 0.8
		decision.Risk = "low"
		decision.Rationale = "This looks like a status, setup, or navigation question rather than a workspace task."
		return decision
	case isAmbiguous(lower):
		decision.Mode = ModeClarify
		decision.Complexity = ComplexitySimple
		decision.Clarification = "Please include the target outcome and any constraints, files, or tools Nomici should use."
		decision.MissingInputs = []string{"target_outcome", "constraints"}
		decision.Confidence = 0.8
		decision.Risk = "low"
		decision.Rationale = "The request is too short to choose a safe execution plan."
		return decision
	}
	if wantsImplementation(lower) || wantsResearch(lower) || wantsPlan(lower) {
		decision.Complexity = ComplexityLongHorizon
		decision.Confidence = 0.7
		decision.Rationale = "The request implies planning, verification, implementation, or research across multiple steps."
	} else {
		decision.Complexity = ComplexityMedium
	}
	if wantsResearch(lower) {
		decision.RequiredTools = append(decision.RequiredTools, "read_project")
		decision.RequiredSkills = append(decision.RequiredSkills, "research")
	}
	if wantsImplementation(lower) {
		decision.RequiredTools = append(decision.RequiredTools, "read_project", "write_project", "run_checks")
		decision.RequiredSkills = append(decision.RequiredSkills, "coding")
		decision.NeedsPlanReview = true
		decision.Risk = "high"
	}
	if wantsMutation(lower) {
		decision.NeedsPlanReview = true
		decision.Risk = "high"
	}
	decision.RequiredTools = unique(decision.RequiredTools)
	decision.RequiredSkills = unique(decision.RequiredSkills)
	return decision
}

func DirectReply(decision RouteDecision) string {
	switch decision.Mode {
	case ModeClarify:
		return decision.Clarification
	case ModeDirectReply:
		return "Nomici is ready to work from Chat. Describe the outcome you want delivered, or open Orchestrate to inspect current sessions and artifacts."
	default:
		return ""
	}
}

func MatchRoles(manifest packs.Manifest, snapshot *graph.Snapshot, startAgentID string, decision RouteDecision) MatchResult {
	result := MatchResult{
		Entrypoint:      startAgentID,
		RequiredTools:   unique(decision.RequiredTools),
		NeedsReview:     decision.NeedsPlanReview,
		SelectionReason: decision.Rationale,
	}
	if snapshot == nil || len(manifest.Roles) == 0 || !contains(manifest.Agents.Entrypoints, startAgentID) {
		return result
	}
	available := map[string]packs.PackRole{}
	for _, role := range manifest.Roles {
		if _, ok := snapshot.IR.Agents[role.ID]; ok {
			available[role.ID] = role
		}
	}
	selectedIDs := roleIDsFromDecision(decision.SelectedRoles, available)
	if len(selectedIDs) == 0 {
		selectedIDs = defaultRoleIDs(manifest, available, decision)
	}
	selectedSet := map[string]bool{}
	for index, id := range selectedIDs {
		role, ok := available[id]
		if !ok || selectedSet[id] {
			continue
		}
		agent := snapshot.IR.Agents[id]
		requiredTools := unique(append(role.RequiredTools, agent.Tools...))
		requiredSkills := unique(append(role.RequiredSkills, agent.Skills...))
		purpose := role.Purpose
		if strings.TrimSpace(agent.Role) != "" {
			purpose = agent.Role
		}
		selectedSet[id] = true
		result.Roles = append(result.Roles, RoleSelection{
			AgentID:         id,
			RoleID:          id,
			Sequence:        index + 1,
			Purpose:         purpose,
			SelectionReason: roleReason(role, decision),
			MatchScore:      roleScore(role, decision),
			RequiredTools:   requiredTools,
			RequiredSkills:  requiredSkills,
			OutputContract:  role.OutputContract,
		})
		result.RequiredTools = unique(append(result.RequiredTools, requiredTools...))
	}
	for _, role := range manifest.Roles {
		if _, ok := available[role.ID]; ok && !selectedSet[role.ID] {
			result.SkippedRoles = append(result.SkippedRoles, SkippedRole{RoleID: role.ID, Reason: "Not required by this route decision."})
		}
	}
	result.RequiredTools = unique(result.RequiredTools)
	return result
}

func defaultAgent(snapshot *graph.Snapshot) string {
	if snapshot == nil {
		return ""
	}
	if _, ok := snapshot.IR.Agents["product_pm"]; ok {
		return "product_pm"
	}
	ids := make([]string, 0, len(snapshot.IR.Agents))
	for id := range snapshot.IR.Agents {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

func defaultRoleIDs(manifest packs.Manifest, available map[string]packs.PackRole, decision RouteDecision) []string {
	ids := []string{}
	appendRole := func(predicate func(packs.PackRole) bool) {
		for _, role := range manifest.Roles {
			if _, ok := available[role.ID]; ok && predicate(role) && !contains(ids, role.ID) {
				ids = append(ids, role.ID)
				return
			}
		}
	}
	appendRole(func(role packs.PackRole) bool { return contains(manifest.Agents.Entrypoints, role.ID) })
	appendRole(func(role packs.PackRole) bool {
		return roleHas(role, "plan") || contains(role.RequiredSkills, "planning")
	})
	if wantsResearch(strings.ToLower(decision.Goal)) {
		appendRole(func(role packs.PackRole) bool {
			return roleHas(role, "research") || contains(role.RequiredSkills, "research") || contains(role.RequiredTools, "read_project")
		})
	}
	if wantsImplementation(strings.ToLower(decision.Goal)) {
		appendRole(func(role packs.PackRole) bool {
			return strings.Contains(strings.ToLower(role.ID), "code") || contains(role.RequiredSkills, "coding") || contains(role.RequiredTools, "write_project")
		})
	}
	appendRole(func(role packs.PackRole) bool {
		return roleHas(role, "report") || contains(role.RequiredSkills, "reporting")
	})
	if len(ids) == 0 {
		for _, role := range manifest.Roles {
			if _, ok := available[role.ID]; ok {
				ids = append(ids, role.ID)
				break
			}
		}
	}
	return ids
}

func roleIDsFromDecision(ids []string, available map[string]packs.PackRole) []string {
	result := []string{}
	for _, id := range ids {
		if _, ok := available[id]; ok && !contains(result, id) {
			result = append(result, id)
		}
	}
	return result
}

func roleHas(role packs.PackRole, text string) bool {
	value := strings.ToLower(role.ID + " " + role.Purpose + " " + role.OutputContract.Kind + " " + role.OutputContract.Description)
	return strings.Contains(value, text)
}

func roleReason(role packs.PackRole, decision RouteDecision) string {
	if contains(decision.SelectedRoles, role.ID) {
		return "Explicitly selected by route decision."
	}
	if contains(role.RequiredTools, "write_project") || contains(role.RequiredSkills, "coding") {
		return "Selected because the request needs implementation or verification."
	}
	if contains(role.RequiredSkills, "research") || contains(role.RequiredTools, "read_project") {
		return "Selected because the request needs project or external facts."
	}
	if roleHas(role, "plan") {
		return "Selected to create a reviewable plan."
	}
	if roleHas(role, "report") {
		return "Selected to produce the final report."
	}
	return "Selected as part of the entrypoint role flow."
}

func roleScore(role packs.PackRole, decision RouteDecision) float64 {
	score := 0.5
	lower := strings.ToLower(decision.Goal)
	if roleHas(role, "plan") && wantsPlan(lower) {
		score += 0.2
	}
	if (contains(role.RequiredSkills, "research") || contains(role.RequiredTools, "read_project")) && wantsResearch(lower) {
		score += 0.25
	}
	if (contains(role.RequiredSkills, "coding") || contains(role.RequiredTools, "write_project")) && wantsImplementation(lower) {
		score += 0.3
	}
	if roleHas(role, "report") {
		score += 0.1
	}
	if score > 1 {
		return 1
	}
	return score
}

func isDirectQuery(lower string) bool {
	direct := []string{"status", "doctor", "setup", "settings", "配置", "设置", "状态", "怎么启动", "how do i", "help"}
	if wantsImplementation(lower) || wantsResearch(lower) || strings.Contains(lower, "plan") || strings.Contains(lower, "计划") {
		return false
	}
	for _, keyword := range direct {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func isAmbiguous(lower string) bool {
	words := strings.Fields(lower)
	return len([]rune(lower)) < 8 || len(words) <= 1
}

func wantsPlan(lower string) bool {
	return containsAny(lower, []string{"plan", "roadmap", "方案", "计划", "拆解", "规划"})
}

func wantsResearch(lower string) bool {
	return containsAny(lower, []string{"research", "compare", "investigate", "analyze", "查", "研究", "对比", "分析", "参考"})
}

func wantsImplementation(lower string) bool {
	return containsAny(lower, []string{"implement", "build", "fix", "change", "edit", "refactor", "test", "ship", "code", "实现", "修复", "改造", "创建", "编排", "测试", "提交", "合并"})
}

func wantsMutation(lower string) bool {
	return containsAny(lower, []string{"write", "edit", "delete", "merge", "commit", "deploy", "改", "写", "删", "提交", "合并", "部署"})
}

func containsAny(value string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func unique(values []string) []string {
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
