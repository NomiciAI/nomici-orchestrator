package orchestration

import (
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/packs"
)

func TestRouteDecisionModes(t *testing.T) {
	snapshot := &graph.Snapshot{IR: graph.IR{Agents: map[string]graph.Agent{"product_pm": {ID: "product_pm"}}}}
	tests := []struct {
		name string
		text string
		mode string
	}{
		{name: "simple setup", text: "setup status", mode: ModeDirectReply},
		{name: "greeting", text: "hey", mode: ModeDirectReply},
		{name: "general chat", text: "I think you should know what to do if I say anything", mode: ModeDirectReply},
		{name: "ambiguous", text: "fix", mode: ModeClarify},
		{name: "implementation", text: "implement the workspace review flow", mode: ModeWorkspaceRun},
		{name: "research", text: "research and compare provider options", mode: ModeWorkspaceRun},
		{name: "repo inspection", text: "Inspect this repo and suggest the next product improvement", mode: ModeWorkspaceRun},
		{name: "fresh price", text: "check BTC price", mode: ModeWorkspaceRun},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision := Route(test.text, "", snapshot)
			if decision.Mode != test.mode {
				t.Fatalf("expected mode %s, got %+v", test.mode, decision)
			}
		})
	}
}

func TestFreshInfoRouteRequiresSearchAndFetch(t *testing.T) {
	decision := Route("check BTC price", "", nil)
	if decision.Mode != ModeWorkspaceRun {
		t.Fatalf("expected workspace run, got %+v", decision)
	}
	if !contains(decision.RequiredTools, "search") || !contains(decision.RequiredTools, "fetch") {
		t.Fatalf("expected search and fetch tools, got %+v", decision.RequiredTools)
	}
	if !contains(decision.RequiredSkills, "research") {
		t.Fatalf("expected research skill, got %+v", decision.RequiredSkills)
	}
}

func TestFreshInfoRouteSelectsResearcher(t *testing.T) {
	manifest := packs.DeveloperTeamManifest()
	snapshot := &graph.Snapshot{IR: graph.IR{Agents: map[string]graph.Agent{
		"product_pm": {ID: "product_pm"},
		"planner":    {ID: "planner"},
		"researcher": {ID: "researcher"},
		"reporter":   {ID: "reporter"},
	}}}
	decision := Route("check BTC price", "", snapshot)
	match := MatchRoles(manifest, snapshot, "product_pm", decision)
	got := []string{}
	for _, role := range match.Roles {
		got = append(got, role.RoleID)
	}
	want := []string{"product_pm", "planner", "researcher", "reporter"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestClarificationUsesNaturalProductLanguage(t *testing.T) {
	decision := Route("fix", "", nil)
	if decision.Mode != ModeClarify {
		t.Fatalf("expected clarify mode, got %+v", decision)
	}
	if decision.Clarification != "What would you like me to help with?" {
		t.Fatalf("expected natural clarification, got %q", decision.Clarification)
	}
	if len(decision.MissingInputs) != 1 || decision.MissingInputs[0] != "goal" {
		t.Fatalf("expected simple missing input, got %+v", decision.MissingInputs)
	}
}

func TestMatchRolesSelectsDynamicSubset(t *testing.T) {
	manifest := packs.DeveloperTeamManifest()
	snapshot := &graph.Snapshot{IR: graph.IR{Agents: map[string]graph.Agent{
		"product_pm": {ID: "product_pm"},
		"planner":    {ID: "planner"},
		"researcher": {ID: "researcher"},
		"coder":      {ID: "coder", Tools: []string{"bash"}, Skills: []string{"testing"}, Role: "Implement and verify changes"},
		"reporter":   {ID: "reporter"},
	}}}
	decision := Route("implement and verify the chat workspace", "", snapshot)
	match := MatchRoles(manifest, snapshot, "product_pm", decision)
	got := []string{}
	for _, role := range match.Roles {
		got = append(got, role.RoleID)
	}
	want := []string{"product_pm", "planner", "coder", "reporter"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
	if len(match.SkippedRoles) == 0 {
		t.Fatalf("expected skipped roles to be visible")
	}
	var coder RoleSelection
	for _, role := range match.Roles {
		if role.RoleID == "coder" {
			coder = role
		}
	}
	if coder.Purpose != "Implement and verify changes" || !contains(coder.RequiredTools, "bash") || !contains(coder.RequiredSkills, "testing") {
		t.Fatalf("expected graph role overrides in match result, got %+v", coder)
	}
}
