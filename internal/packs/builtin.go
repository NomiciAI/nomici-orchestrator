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
		Description: "A first-run developer team with a product PM entrypoint and architecture subagent.",
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
			Includes:    []string{"product_pm", "architect"},
			Optional:    []string{"implementer", "reviewer", "test_runner"},
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
