package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/clirunner"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	"github.com/NomiciAI/nomici-orchestrator/internal/sharedcontext"
)

func padValue(value string) string {
	if value == "" {
		return ""
	}
	return " " + value
}

func graphModelToProvider(model graph.Model) *providers.Profile {
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

func graphPrompt(agent graph.Agent, prompt string) string {
	parts := []string{}
	if agent.Role != "" {
		parts = append(parts, "Role:\n"+agent.Role)
	}
	if agent.Instructions != "" {
		parts = append(parts, "Instructions:\n"+agent.Instructions)
	}
	parts = append(parts, "Task:\n"+prompt)
	return strings.Join(parts, "\n\n")
}

func cliRunnerConfig(runtime graph.Runtime, agent graph.Agent, configPath string) clirunner.Config {
	return clirunner.Config{
		RuntimeID:      runtime.ID,
		AgentID:        agent.ID,
		Workspace:      resolveRuntimeWorkspace(runtime.Workspace, configPath),
		Executable:     runtime.Invoke.Executable,
		Args:           runtime.Invoke.Args,
		Stdin:          runtime.Invoke.Stdin,
		Env:            runtime.Env,
		EnvFrom:        runtime.EnvFrom,
		TimeoutSeconds: runtime.TimeoutSeconds,
		FilesWrite:     runtimeFilesWrite(runtime),
	}
}

func resolveRuntimeWorkspace(workspace string, configPath string) string {
	if strings.TrimSpace(workspace) == "" {
		workspace = "."
	}
	if filepath.IsAbs(workspace) {
		return workspace
	}
	configDir := filepath.Dir(configPath)
	if configDir == "." || configDir == "" {
		return workspace
	}
	return filepath.Join(configDir, workspace)
}

func runtimeFilesWrite(runtime graph.Runtime) bool {
	value, ok := runtime.Capabilities["files_write"]
	if !ok {
		return true
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true") || strings.EqualFold(typed, "yes")
	default:
		return true
	}
}

func cliArtifacts(result *clirunner.Result) []map[string]string {
	artifacts := []map[string]string{}
	add := func(kind string, path string) {
		if path == "" {
			return
		}
		artifacts = append(artifacts, map[string]string{
			"kind": kind,
			"path": path,
		})
	}
	add("stdout", result.StdoutRef)
	add("stderr", result.StderrRef)
	add("pre_diff", result.PreDiffRef)
	add("diff", result.DiffRef)
	return artifacts
}

func jsonPayload(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("{}")
	}
	return payload
}

func oneLine(value string, max int) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
	if len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}

func displayOutput(value string, max int) string {
	return oneLine(sharedcontext.RedactText(value), max)
}
