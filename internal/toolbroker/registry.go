package toolbroker

import (
	"fmt"
	"sort"
)

func Definitions() []Definition {
	definitions := []Definition{
		{
			ID:             ToolListFiles,
			Description:    "List files inside the session workspace.",
			Auth:           "none",
			NetworkRisk:    RiskLow,
			FilesystemRisk: "read",
			MutationRisk:   RiskLow,
			AllowedScopes:  []string{"workspace"},
			RedactionRules: []string{"paths_only"},
			Execution:      "local_sandbox",
		},
		{
			ID:             ToolReadFile,
			Description:    "Read a file inside the session workspace.",
			Auth:           "none",
			NetworkRisk:    RiskLow,
			FilesystemRisk: "read",
			MutationRisk:   RiskLow,
			AllowedScopes:  []string{"workspace"},
			RedactionRules: []string{"content_preview"},
			Execution:      "local_sandbox",
		},
		{
			ID:             ToolWriteFile,
			Description:    "Write a file inside the session workspace after approval.",
			Auth:           "approval",
			NetworkRisk:    RiskLow,
			FilesystemRisk: "write",
			MutationRisk:   RiskMedium,
			AllowedScopes:  []string{"workspace"},
			RedactionRules: []string{"content_preview"},
			Execution:      "local_sandbox",
		},
		{
			ID:             ToolReplaceFile,
			Description:    "Replace text in a workspace file after approval.",
			Auth:           "approval",
			NetworkRisk:    RiskLow,
			FilesystemRisk: "write",
			MutationRisk:   RiskMedium,
			AllowedScopes:  []string{"workspace"},
			RedactionRules: []string{"content_preview"},
			Execution:      "local_sandbox",
		},
		{
			ID:             ToolPresentArtifact,
			Description:    "Register a file or text preview as a session artifact after approval.",
			Auth:           "approval",
			NetworkRisk:    RiskLow,
			FilesystemRisk: "write",
			MutationRisk:   RiskMedium,
			AllowedScopes:  []string{"workspace", "artifacts"},
			RedactionRules: []string{"content_preview"},
			Execution:      "local_sandbox",
		},
		{
			ID:             ToolBash,
			Description:    "Run a shell command in the session workspace after approval.",
			Auth:           "approval",
			NetworkRisk:    RiskMedium,
			FilesystemRisk: "read_write",
			MutationRisk:   RiskHigh,
			AllowedScopes:  []string{"workspace"},
			RedactionRules: []string{"stdout_stderr_preview"},
			Execution:      "local_sandbox",
		},
		{
			ID:             ToolSearch,
			Description:    "Run a configured web search query.",
			Auth:           "provider_config",
			NetworkRisk:    RiskMedium,
			FilesystemRisk: "none",
			MutationRisk:   RiskLow,
			AllowedScopes:  []string{"network"},
			RedactionRules: []string{"query_preview"},
			Execution:      "network_read",
		},
		{
			ID:             ToolFetch,
			Description:    "Fetch a configured web page reader result.",
			Auth:           "provider_config",
			NetworkRisk:    RiskMedium,
			FilesystemRisk: "none",
			MutationRisk:   RiskLow,
			AllowedScopes:  []string{"network"},
			RedactionRules: []string{"url_preview"},
			Execution:      "network_read",
		},
	}
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].ID < definitions[j].ID })
	return definitions
}

func DefinitionByID(id string) (Definition, error) {
	for _, definition := range Definitions() {
		if definition.ID == id {
			return definition, nil
		}
	}
	return Definition{}, fmt.Errorf("tool %q was not found", id)
}

func RequiresApproval(toolID string) bool {
	switch toolID {
	case ToolWriteFile, ToolReplaceFile, ToolPresentArtifact, ToolBash:
		return true
	default:
		return false
	}
}
