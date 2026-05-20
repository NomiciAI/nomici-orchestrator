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
			Parameters: objectSchema(map[string]any{
				"path":  stringSchema("Workspace-relative directory path. Defaults to ."),
				"limit": numberSchema("Maximum number of entries to return."),
			}, []string{}),
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
			Parameters: objectSchema(map[string]any{
				"path":      stringSchema("Workspace-relative file path."),
				"max_bytes": numberSchema("Maximum bytes to read."),
			}, []string{"path"}),
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
			Parameters: objectSchema(map[string]any{
				"path":    stringSchema("Workspace-relative file path."),
				"content": stringSchema("Full replacement content for the file."),
			}, []string{"path", "content"}),
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
			Parameters: objectSchema(map[string]any{
				"path":         stringSchema("Workspace-relative file path."),
				"old":          stringSchema("Text to replace."),
				"new":          stringSchema("Replacement text."),
				"max_replaces": numberSchema("Maximum replacements to apply."),
			}, []string{"path", "old", "new"}),
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
			Parameters: objectSchema(map[string]any{
				"title":   stringSchema("Artifact title."),
				"type":    stringSchema("Artifact type such as report, plan, file, or diff."),
				"path":    stringSchema("Optional workspace-relative file path to present."),
				"preview": stringSchema("Optional text preview when no file is present."),
			}, []string{"title"}),
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
			Parameters: objectSchema(map[string]any{
				"command":         stringSchema("Shell command to run in the session workspace."),
				"cwd":             stringSchema("Workspace-relative working directory. Defaults to ."),
				"timeout_seconds": numberSchema("Timeout in seconds."),
			}, []string{"command"}),
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
			Parameters: objectSchema(map[string]any{
				"query": stringSchema("Search query."),
				"limit": numberSchema("Maximum number of results."),
			}, []string{"query"}),
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
			Parameters: objectSchema(map[string]any{
				"url":       stringSchema("HTTP or HTTPS URL to fetch."),
				"max_bytes": numberSchema("Maximum bytes to keep in the preview."),
			}, []string{"url"}),
		},
	}
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].ID < definitions[j].ID })
	return definitions
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func numberSchema(description string) map[string]any {
	return map[string]any{"type": "number", "description": description}
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
