package packs

import "fmt"

const DeveloperTeamID = "developer-team"

func ListBuiltins() []Manifest {
	return []Manifest{DeveloperTeamManifest()}
}

func GetBuiltin(id string) (Manifest, bool) {
	switch id {
	case DeveloperTeamID:
		return DeveloperTeamManifest(), true
	default:
		return Manifest{}, false
	}
}

func DeveloperTeamManifest() Manifest {
	return Manifest{
		ID:          DeveloperTeamID,
		Name:        "Developer Team",
		Version:     "0.1.0",
		Kind:        "agent_pack",
		Description: "A first-run developer team with coordinator, planner, researcher, coder, and reporter roles.",
		Publisher:   "NomiciAI",
		License:     "Apache-2.0",
		Requires: map[string]string{
			"nomici": ">=0.1.0",
		},
		Permissions: Permissions{
			Filesystem: FilesystemPermissions{
				Read:  []string{"./workspace"},
				Write: []string{"./workspace"},
			},
			Shell: ShellPermissions{Mode: "approval"},
		},
		Agents: PackAgents{
			Entrypoints: []string{"product_pm"},
			Includes:    []string{"product_pm", "planner", "researcher", "coder", "reporter"},
			Optional:    []string{"architect", "implementer", "reviewer", "test_runner"},
		},
		Roles: []PackRole{
			{
				ID:              "product_pm",
				Purpose:         "Coordinate the run, keep the user goal explicit, and decide the ordered handoff path.",
				RequiredSkills:  []string{"planning"},
				ModelPreference: "default",
				HandoffMode:     "sequential",
				OutputContract: OutputContract{
					Kind:        "coordination_brief",
					Description: "A concise run brief with the requested outcome, risks, assumptions, and role sequence.",
					Required:    []string{"goal", "role_sequence", "risks"},
				},
			},
			{
				ID:              "planner",
				Purpose:         "Turn the goal into bounded phases, acceptance criteria, dependencies, and open questions.",
				RequiredSkills:  []string{"planning"},
				ModelPreference: "default",
				HandoffMode:     "sequential",
				OutputContract: OutputContract{
					Kind:        "implementation_plan",
					Description: "A plan that is specific enough for the next role to execute without choosing scope.",
					Required:    []string{"phases", "acceptance_criteria", "dependencies"},
				},
			},
			{
				ID:              "researcher",
				Purpose:         "Gather project or external facts needed before implementation choices are made.",
				RequiredTools:   []string{"read_project"},
				RequiredSkills:  []string{"research"},
				ModelPreference: "default",
				HandoffMode:     "sequential",
				OutputContract: OutputContract{
					Kind:        "research_summary",
					Description: "Verified findings, confidence, contradictions, and follow-up checks.",
					Required:    []string{"findings", "confidence", "open_checks"},
				},
			},
			{
				ID:                "coder",
				Purpose:           "Produce implementation-oriented guidance and execute code changes when tools allow it.",
				RequiredTools:     []string{"read_project", "write_project", "run_checks"},
				RequiredSkills:    []string{"coding"},
				ModelPreference:   "default",
				RuntimePreference: "local",
				HandoffMode:       "sequential",
				OutputContract: OutputContract{
					Kind:        "implementation_result",
					Description: "Changed behavior, touched surfaces, tests, and remaining risks.",
					Required:    []string{"changes", "verification", "risks"},
				},
			},
			{
				ID:              "reporter",
				Purpose:         "Summarize completed work, verification evidence, residual risks, and next steps.",
				RequiredSkills:  []string{"reporting"},
				ModelPreference: "default",
				HandoffMode:     "sequential",
				OutputContract: OutputContract{
					Kind:        "final_report",
					Description: "A concise user-facing report tied to trace, artifacts, and task outcomes.",
					Required:    []string{"summary", "verification", "residual_risks"},
				},
			},
		},
		Trust: Trust{Level: "official"},
	}
}

func RequireBuiltin(id string) (Manifest, error) {
	manifest, ok := GetBuiltin(id)
	if !ok {
		return Manifest{}, fmt.Errorf("pack %q was not found; run `nomici pack list` to see bundled packs", id)
	}
	return manifest, nil
}
