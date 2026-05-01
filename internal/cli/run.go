package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/clirunner"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	tracepkg "github.com/NomiciAI/nomici-orchestrator/internal/trace"
	"github.com/spf13/cobra"
)

func newRunCommand() *cobra.Command {
	var gatewayURL string
	var configPath string
	var dbPath string
	command := &cobra.Command{
		Use:   "run [entrypoint] [prompt]",
		Short: "Run implemented Nomici proof-slice commands",
		Args:  cobra.MinimumNArgs(0),
		RunE: func(command *cobra.Command, args []string) error {
			if len(args) == 0 {
				return command.Help()
			}
			prompt := "Say hello from Nomici."
			if len(args) > 1 {
				prompt = strings.Join(args[1:], " ")
			}
			if envURL := os.Getenv("NOMICI_GATEWAY_URL"); envURL != "" && gatewayURL == defaultGatewayURL {
				gatewayURL = envURL
			}
			return runGraphEntrypoint(command, configPath, dbPath, gatewayURL, args[0], prompt)
		},
	}
	command.PersistentFlags().StringVar(&gatewayURL, "gateway-url", defaultGatewayURL, "Nomici Gateway URL")
	command.PersistentFlags().StringVar(&configPath, "config", "nomici.yaml", "AgentSpec config path")
	command.PersistentFlags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.AddCommand(newRunModelCommand(&gatewayURL))
	return command
}

func newRunModelCommand(gatewayURL *string) *cobra.Command {
	return &cobra.Command{
		Use:   "model <profile_id> [prompt]",
		Short: "Run a model profile through Nomici Gateway",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			prompt := "Say hello from Nomici."
			if len(args) > 1 {
				prompt = strings.Join(args[1:], " ")
			}
			if envURL := os.Getenv("NOMICI_GATEWAY_URL"); envURL != "" && *gatewayURL == defaultGatewayURL {
				*gatewayURL = envURL
			}

			result, err := postModelTest(command.Context(), *gatewayURL, args[0], prompt)
			if err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Run ID:    %s\n", result.RunID)
			fmt.Fprintf(command.OutOrStdout(), "Status:    %s\n", result.Status)
			if len(result.Messages) > 0 {
				fmt.Fprintf(command.OutOrStdout(), "Response:  %s\n", result.Messages[0].Content)
			}
			return nil
		},
	}
}

func runGraphEntrypoint(command *cobra.Command, configPath string, dbPath string, gatewayURL string, entrypoint string, prompt string) error {
	snapshot, err := compileGraphFromConfig(configPath, command)
	if err != nil {
		return err
	}
	agent, ok := snapshot.IR.Agents[entrypoint]
	if !ok {
		return fmt.Errorf("entrypoint %q was not found in AgentGraph", entrypoint)
	}
	if agent.Kind != agentspec.AgentKindGateway && agent.Kind != agentspec.AgentKindModel {
		if agent.Kind != agentspec.AgentKindExternal {
			return fmt.Errorf("agent %q has kind %q, which is not executable in Gate 4; gateway_agent, model_agent, and external_agent backed by cli_agent are supported", entrypoint, agent.Kind)
		}
	}
	for _, edge := range snapshot.IR.Edges {
		if edge.From == entrypoint {
			return fmt.Errorf("agent %q has outgoing %q edge to %q; multi-node graph execution is not implemented in Gate 4", entrypoint, edge.Mode, edge.To)
		}
	}

	db, err := openMigratedDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := graph.NewStore(db).Save(command.Context(), snapshot); err != nil {
		return err
	}

	if agent.Kind == agentspec.AgentKindExternal {
		return runExternalCLIAgent(command, snapshot, agent, configPath, prompt, db)
	}

	model, ok := snapshot.IR.Models[agent.Model]
	if !ok {
		return fmt.Errorf("agent %q references missing compiled model %q", entrypoint, agent.Model)
	}
	if err := providers.NewStore(db).Save(command.Context(), graphModelToProvider(model)); err != nil {
		return err
	}

	result, err := postModelTest(command.Context(), gatewayURL, model.ID, graphPrompt(agent, prompt))
	if err != nil {
		return err
	}
	fmt.Fprintf(command.OutOrStdout(), "Graph:     %s\n", snapshot.SnapshotID)
	fmt.Fprintf(command.OutOrStdout(), "Agent:     %s\n", agent.ID)
	fmt.Fprintf(command.OutOrStdout(), "Run ID:    %s\n", result.RunID)
	fmt.Fprintf(command.OutOrStdout(), "Status:    %s\n", result.Status)
	if len(result.Messages) > 0 {
		fmt.Fprintf(command.OutOrStdout(), "Response:  %s\n", result.Messages[0].Content)
	}
	return nil
}

func runExternalCLIAgent(command *cobra.Command, snapshot *graph.Snapshot, agent graph.Agent, configPath string, prompt string, db *sql.DB) error {
	runtime, ok := snapshot.IR.Runtimes[agent.Runtime]
	if !ok {
		return fmt.Errorf("agent %q references missing compiled runtime %q", agent.ID, agent.Runtime)
	}
	if runtime.Kind != agentspec.RuntimeKindCLIAgent {
		return fmt.Errorf("agent %q uses runtime %q with kind %q; only cli_agent external runtimes are executable in Gate 4", agent.ID, runtime.ID, runtime.Kind)
	}

	traceStore := tracepkg.NewStore(db)
	runID := ids.New("run")
	taskID := ids.New("task")
	ctx := command.Context()

	if err := traceStore.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventRunStarted,
		NodeID:    agent.ID,
		RuntimeID: runtime.ID,
		Payload: jsonPayload(map[string]any{
			"graph_id": snapshot.SnapshotID,
			"agent_id": agent.ID,
			"task_id":  taskID,
		}),
	}); err != nil {
		return err
	}
	if err := traceStore.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventAdapterInvoked,
		NodeID:    agent.ID,
		RuntimeID: runtime.ID,
		Payload: jsonPayload(map[string]any{
			"runtime_kind": runtime.Kind,
			"runner":       runtime.Runner,
			"workspace":    runtime.Workspace,
			"executable":   runtime.Invoke.Executable,
			"args_count":   len(runtime.Invoke.Args),
			"env_from":     runtime.EnvFrom,
		}),
	}); err != nil {
		return err
	}

	result, err := clirunner.Invoke(ctx, cliRunnerConfig(runtime, agent, configPath), clirunner.Request{
		RunID:  runID,
		TaskID: taskID,
		Prompt: graphPrompt(agent, prompt),
	})
	if err != nil {
		_ = appendExternalFailure(ctx, traceStore, runID, agent.ID, runtime.ID, err.Error())
		return err
	}

	for _, artifact := range cliArtifacts(result) {
		if err := traceStore.Append(ctx, &tracepkg.Event{
			RunID:     runID,
			Type:      tracepkg.EventArtifactCreated,
			NodeID:    agent.ID,
			RuntimeID: runtime.ID,
			Payload:   jsonPayload(artifact),
		}); err != nil {
			return err
		}
	}

	completionPayload := map[string]any{
		"status":        result.Status,
		"exit_code":     result.ExitCode,
		"changed_files": result.ChangedFiles,
		"stdout_ref":    result.StdoutRef,
		"stderr_ref":    result.StderrRef,
		"diff_ref":      result.DiffRef,
	}
	if result.Error != "" {
		completionPayload["error"] = result.Error
	}
	eventType := tracepkg.EventAdapterCompleted
	runType := tracepkg.EventRunCompleted
	if result.Status != clirunner.StatusCompleted {
		eventType = tracepkg.EventAdapterFailed
		runType = tracepkg.EventRunFailed
	}
	if err := traceStore.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      eventType,
		NodeID:    agent.ID,
		RuntimeID: runtime.ID,
		Payload:   jsonPayload(completionPayload),
	}); err != nil {
		return err
	}
	if err := traceStore.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      runType,
		NodeID:    agent.ID,
		RuntimeID: runtime.ID,
	}); err != nil {
		return err
	}

	fmt.Fprintf(command.OutOrStdout(), "Graph:     %s\n", snapshot.SnapshotID)
	fmt.Fprintf(command.OutOrStdout(), "Agent:     %s\n", agent.ID)
	fmt.Fprintf(command.OutOrStdout(), "Runtime:   %s\n", runtime.ID)
	fmt.Fprintf(command.OutOrStdout(), "Run ID:    %s\n", runID)
	fmt.Fprintf(command.OutOrStdout(), "Status:    %s\n", result.Status)
	if result.Error != "" {
		fmt.Fprintf(command.OutOrStdout(), "Error:     %s\n", result.Error)
	}
	if strings.TrimSpace(result.Stdout) != "" {
		fmt.Fprintf(command.OutOrStdout(), "Response:  %s\n", oneLine(result.Stdout, 500))
	}
	if result.DiffRef != "" {
		fmt.Fprintf(command.OutOrStdout(), "Diff:      %s\n", result.DiffRef)
	}
	if len(result.ChangedFiles) > 0 {
		fmt.Fprintf(command.OutOrStdout(), "Changed:   %s\n", strings.Join(result.ChangedFiles, ", "))
	}
	if result.Status != clirunner.StatusCompleted {
		return fmt.Errorf("cli_agent run failed: %s", result.Error)
	}
	return nil
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

func appendExternalFailure(ctx context.Context, traceStore *tracepkg.Store, runID string, agentID string, runtimeID string, message string) error {
	if err := traceStore.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventAdapterFailed,
		NodeID:    agentID,
		RuntimeID: runtimeID,
		Payload: jsonPayload(map[string]string{
			"message": message,
		}),
	}); err != nil {
		return err
	}
	return traceStore.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventRunFailed,
		NodeID:    agentID,
		RuntimeID: runtimeID,
	})
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
