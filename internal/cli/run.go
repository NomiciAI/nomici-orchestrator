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
	"github.com/NomiciAI/nomici-orchestrator/internal/policy"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	"github.com/NomiciAI/nomici-orchestrator/internal/sharedcontext"
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
	outgoing := outgoingEdges(snapshot, entrypoint)

	db, err := openMigratedDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := graph.NewStore(db).Save(command.Context(), snapshot); err != nil {
		return err
	}

	if len(outgoing) > 0 {
		if agent.Kind == agentspec.AgentKindExternal && len(outgoing) == 1 && outgoing[0].Mode == "handoff" {
			return runExternalCLIHandoff(command, snapshot, agent, outgoing[0], configPath, prompt, db)
		}
		return fmt.Errorf("agent %q has outgoing graph edges; Gate 5 only supports one handoff edge between cli_agent-backed external_agent nodes", entrypoint)
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
	contextStore := sharedcontext.NewStore(db)
	policyService := policy.NewService(db)
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

	result, err := invokeExternalCLIAgent(ctx, traceStore, policyService, snapshot.ProjectID, runtime, agent, configPath, runID, taskID, graphPrompt(agent, prompt), nil)
	if err != nil {
		_ = appendExternalFailure(ctx, traceStore, runID, agent.ID, runtime.ID, err.Error())
		return err
	}

	contextSnapshot, err := saveCLIContextSnapshot(ctx, contextStore, traceStore, snapshot.ProjectID, runID, taskID, agent.ID, "", result, sharedcontext.KindRunSummary)
	if err != nil {
		return err
	}

	runType := tracepkg.EventRunCompleted
	if result.Status != clirunner.StatusCompleted {
		runType = tracepkg.EventRunFailed
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
	if contextSnapshot != nil {
		fmt.Fprintf(command.OutOrStdout(), "Context:   %s\n", contextSnapshot.SnapshotID)
	}
	if result.Error != "" {
		fmt.Fprintf(command.OutOrStdout(), "Error:     %s\n", result.Error)
	}
	if strings.TrimSpace(result.Stdout) != "" {
		fmt.Fprintf(command.OutOrStdout(), "Response:  %s\n", displayOutput(result.Stdout, 500))
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

func runExternalCLIHandoff(command *cobra.Command, snapshot *graph.Snapshot, fromAgent graph.Agent, edge graph.Edge, configPath string, prompt string, db *sql.DB) error {
	toAgent, ok := snapshot.IR.Agents[edge.To]
	if !ok {
		return fmt.Errorf("handoff target %q was not found in compiled graph", edge.To)
	}
	if toAgent.Kind != agentspec.AgentKindExternal {
		return fmt.Errorf("Gate 5 handoff target %q has kind %q; only external_agent targets are supported", toAgent.ID, toAgent.Kind)
	}
	fromRuntime, err := cliRuntimeForAgent(snapshot, fromAgent)
	if err != nil {
		return err
	}
	toRuntime, err := cliRuntimeForAgent(snapshot, toAgent)
	if err != nil {
		return err
	}

	traceStore := tracepkg.NewStore(db)
	contextStore := sharedcontext.NewStore(db)
	policyService := policy.NewService(db)
	runID := ids.New("run")
	taskID := ids.New("task")
	ctx := command.Context()

	if err := traceStore.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventRunStarted,
		NodeID:    fromAgent.ID,
		RuntimeID: fromRuntime.ID,
		Payload: jsonPayload(map[string]any{
			"graph_id": snapshot.SnapshotID,
			"agent_id": fromAgent.ID,
			"task_id":  taskID,
		}),
	}); err != nil {
		return err
	}

	first, err := invokeExternalCLIAgent(ctx, traceStore, policyService, snapshot.ProjectID, fromRuntime, fromAgent, configPath, runID, taskID, graphPrompt(fromAgent, prompt), nil)
	if err != nil {
		_ = appendExternalFailure(ctx, traceStore, runID, fromAgent.ID, fromRuntime.ID, err.Error())
		return err
	}
	handoffSnapshot, err := saveCLIContextSnapshot(ctx, contextStore, traceStore, snapshot.ProjectID, runID, taskID, fromAgent.ID, toAgent.ID, first, sharedcontext.KindHandoffBriefing)
	if err != nil {
		return err
	}
	if err := traceStore.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventHandoffCreated,
		NodeID:    fromAgent.ID,
		RuntimeID: fromRuntime.ID,
		Payload: jsonPayload(map[string]any{
			"from":        fromAgent.ID,
			"to":          toAgent.ID,
			"edge_id":     edge.ID,
			"mode":        edge.Mode,
			"snapshot_id": handoffSnapshot.SnapshotID,
		}),
	}); err != nil {
		return err
	}
	if err := traceStore.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventHandoffContextAttached,
		NodeID:    toAgent.ID,
		RuntimeID: toRuntime.ID,
		Payload: jsonPayload(map[string]any{
			"from":        fromAgent.ID,
			"to":          toAgent.ID,
			"snapshot_id": handoffSnapshot.SnapshotID,
		}),
	}); err != nil {
		return err
	}

	runType := tracepkg.EventRunCompleted
	second := (*clirunner.Result)(nil)
	if first.Status == clirunner.StatusCompleted {
		if err := traceStore.Append(ctx, &tracepkg.Event{
			RunID:     runID,
			Type:      tracepkg.EventHandoffAccepted,
			NodeID:    toAgent.ID,
			RuntimeID: toRuntime.ID,
			Payload: jsonPayload(map[string]any{
				"from":        fromAgent.ID,
				"to":          toAgent.ID,
				"snapshot_id": handoffSnapshot.SnapshotID,
			}),
		}); err != nil {
			return err
		}
		second, err = invokeExternalCLIAgent(ctx, traceStore, policyService, snapshot.ProjectID, toRuntime, toAgent, configPath, runID, taskID, graphPrompt(toAgent, prompt), handoffSnapshot)
		if err != nil {
			_ = appendExternalFailure(ctx, traceStore, runID, toAgent.ID, toRuntime.ID, err.Error())
			return err
		}
		if _, err := saveCLIContextSnapshot(ctx, contextStore, traceStore, snapshot.ProjectID, runID, taskID, toAgent.ID, "", second, sharedcontext.KindRunSummary); err != nil {
			return err
		}
		if second.Status != clirunner.StatusCompleted {
			runType = tracepkg.EventRunFailed
		}
	} else {
		runType = tracepkg.EventRunFailed
	}

	if err := traceStore.Append(ctx, &tracepkg.Event{
		RunID: runID,
		Type:  runType,
	}); err != nil {
		return err
	}

	fmt.Fprintf(command.OutOrStdout(), "Graph:     %s\n", snapshot.SnapshotID)
	fmt.Fprintf(command.OutOrStdout(), "Handoff:   %s -> %s\n", fromAgent.ID, toAgent.ID)
	fmt.Fprintf(command.OutOrStdout(), "Run ID:    %s\n", runID)
	fmt.Fprintf(command.OutOrStdout(), "Context:   %s\n", handoffSnapshot.SnapshotID)
	fmt.Fprintf(command.OutOrStdout(), "Status:    %s\n", handoffRunStatus(first, second))
	if strings.TrimSpace(first.Stdout) != "" {
		fmt.Fprintf(command.OutOrStdout(), "Upstream:  %s\n", displayOutput(first.Stdout, 500))
	}
	if second != nil && strings.TrimSpace(second.Stdout) != "" {
		fmt.Fprintf(command.OutOrStdout(), "Downstream:%s\n", padValue(displayOutput(second.Stdout, 500)))
	}
	if first.Status != clirunner.StatusCompleted {
		return fmt.Errorf("upstream cli_agent run failed: %s", first.Error)
	}
	if second != nil && second.Status != clirunner.StatusCompleted {
		return fmt.Errorf("downstream cli_agent run failed: %s", second.Error)
	}
	return nil
}

func invokeExternalCLIAgent(ctx context.Context, traceStore *tracepkg.Store, policyService *policy.Service, projectID string, runtime graph.Runtime, agent graph.Agent, configPath string, runID string, taskID string, prompt string, upstream *sharedcontext.Snapshot) (*clirunner.Result, error) {
	briefing := sharedcontext.RenderBriefing(upstream)
	policyDecision, err := checkCLIAgentPolicy(ctx, traceStore, policyService, projectID, runtime, agent, configPath, runID)
	if err != nil {
		return nil, err
	}
	if policyDecision.Decision != policy.DecisionAllow {
		return nil, policyBlockedError(policyDecision)
	}
	if err := traceStore.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventAdapterInvoked,
		NodeID:    agent.ID,
		RuntimeID: runtime.ID,
		Payload: jsonPayload(map[string]any{
			"runtime_kind":       runtime.Kind,
			"runner":             runtime.Runner,
			"workspace":          runtime.Workspace,
			"executable":         runtime.Invoke.Executable,
			"args_count":         len(runtime.Invoke.Args),
			"env_from":           runtime.EnvFrom,
			"shared_context_ref": briefing.SnapshotID,
		}),
	}); err != nil {
		return nil, err
	}

	result, err := clirunner.Invoke(ctx, cliRunnerConfig(runtime, agent, configPath), clirunner.Request{
		RunID:  runID,
		TaskID: taskID,
		Prompt: prompt,
		SharedContext: clirunner.SharedContext{
			SnapshotID: briefing.SnapshotID,
			Briefing:   briefing.Text,
		},
	})
	if err != nil {
		return nil, err
	}
	for _, artifact := range cliArtifacts(result) {
		if err := traceStore.Append(ctx, &tracepkg.Event{
			RunID:     runID,
			Type:      tracepkg.EventArtifactCreated,
			NodeID:    agent.ID,
			RuntimeID: runtime.ID,
			Payload:   jsonPayload(artifact),
		}); err != nil {
			return nil, err
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
	if result.Status != clirunner.StatusCompleted {
		eventType = tracepkg.EventAdapterFailed
	}
	if err := traceStore.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      eventType,
		NodeID:    agent.ID,
		RuntimeID: runtime.ID,
		Payload:   jsonPayload(completionPayload),
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func checkCLIAgentPolicy(ctx context.Context, traceStore *tracepkg.Store, policyService *policy.Service, projectID string, runtime graph.Runtime, agent graph.Agent, configPath string, runID string) (*policy.Decision, error) {
	workspace := resolveRuntimeWorkspace(runtime.Workspace, configPath)
	action := policy.ActionRequest{
		RunID:      runID,
		ActionID:   ids.New("action"),
		ActionType: policy.ActionCLIInvoke,
		ProjectID:  projectID,
		AgentID:    agent.ID,
		RuntimeID:  runtime.ID,
		Workspace:  workspace,
		FilesWrite: runtimeFilesWrite(runtime),
		Summary:    policySummary(runtime, agent, workspace),
		Subject: map[string]string{
			"workspace": workspace,
			"agent_id":  agent.ID,
			"runtime":   runtime.ID,
		},
	}
	decision, err := policyService.Check(ctx, action)
	if err != nil {
		return nil, err
	}
	if err := appendPolicyTrace(ctx, traceStore, runID, agent.ID, runtime.ID, action, decision); err != nil {
		return nil, err
	}
	return decision, nil
}

func appendPolicyTrace(ctx context.Context, traceStore *tracepkg.Store, runID string, agentID string, runtimeID string, action policy.ActionRequest, decision *policy.Decision) error {
	payload := jsonPayload(map[string]any{
		"decision":    decision.Decision,
		"risk":        decision.Risk,
		"reason":      decision.Reason,
		"approval_id": decision.ApprovalID,
		"scope":       decision.Scope,
		"action_type": action.ActionType,
		"summary":     action.Summary,
		"workspace":   action.Workspace,
		"fingerprint": decision.Fingerprint,
		"files_write": action.FilesWrite,
	})
	if err := traceStore.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventPolicyChecked,
		NodeID:    agentID,
		RuntimeID: runtimeID,
		Payload:   payload,
	}); err != nil {
		return err
	}
	switch decision.Decision {
	case policy.DecisionApproval:
		if err := traceStore.Append(ctx, &tracepkg.Event{
			RunID:     runID,
			Type:      tracepkg.EventApprovalRequested,
			NodeID:    agentID,
			RuntimeID: runtimeID,
			Payload:   payload,
		}); err != nil {
			return err
		}
		return traceStore.Append(ctx, &tracepkg.Event{
			RunID:     runID,
			Type:      tracepkg.EventPolicyBlocked,
			NodeID:    agentID,
			RuntimeID: runtimeID,
			Payload:   payload,
		})
	case policy.DecisionDeny:
		return traceStore.Append(ctx, &tracepkg.Event{
			RunID:     runID,
			Type:      tracepkg.EventPolicyBlocked,
			NodeID:    agentID,
			RuntimeID: runtimeID,
			Payload:   payload,
		})
	default:
		return nil
	}
}

func policyBlockedError(decision *policy.Decision) error {
	switch decision.Decision {
	case policy.DecisionApproval:
		return fmt.Errorf("approval required: %s. Approval: %s. Remediation: run `nomici approvals grant %s --scope once` and rerun the command, or use `--scope run` for matching actions in the next run", decision.Reason, decision.ApprovalID, decision.ApprovalID)
	case policy.DecisionDeny:
		return fmt.Errorf("policy denied action: %s", decision.Reason)
	default:
		return fmt.Errorf("policy blocked action: %s", decision.Reason)
	}
}

func policySummary(runtime graph.Runtime, agent graph.Agent, workspace string) string {
	if runtimeFilesWrite(runtime) {
		return fmt.Sprintf("Run mutable cli_agent %s for agent %s in %s", runtime.ID, agent.ID, workspace)
	}
	return fmt.Sprintf("Run read-only cli_agent %s for agent %s in %s", runtime.ID, agent.ID, workspace)
}

func saveCLIContextSnapshot(ctx context.Context, contextStore *sharedcontext.Store, traceStore *tracepkg.Store, projectID string, runID string, taskID string, fromAgent string, toAgent string, result *clirunner.Result, kind string) (*sharedcontext.Snapshot, error) {
	snapshot := &sharedcontext.Snapshot{
		ProjectID:       projectID,
		RunID:           runID,
		TaskID:          taskID,
		FromAgent:       fromAgent,
		ToAgent:         toAgent,
		Summary:         cliContextSummary(result),
		Decisions:       cliContextDecisions(result),
		OpenIssues:      cliOpenIssues(result),
		Recommendations: cliRecommendations(toAgent, result),
		ArtifactRefs:    cliArtifactRefs(result),
		CreatedBy:       sharedcontext.CreatedBy{Kind: "gateway_generated", AgentID: fromAgent},
	}
	if kind == sharedcontext.KindHandoffBriefing && toAgent != "" {
		snapshot.ContextItemRefs = []string{}
	}
	if err := contextStore.SaveSnapshot(ctx, snapshot); err != nil {
		return nil, err
	}
	if err := traceStore.Append(ctx, &tracepkg.Event{
		RunID:   runID,
		Type:    tracepkg.EventContextSnapshotCreated,
		NodeID:  fromAgent,
		Payload: jsonPayload(contextSnapshotPayload(snapshot, kind)),
	}); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func contextSnapshotPayload(snapshot *sharedcontext.Snapshot, kind string) map[string]any {
	return map[string]any{
		"snapshot_id":   snapshot.SnapshotID,
		"kind":          kind,
		"from_agent":    snapshot.FromAgent,
		"to_agent":      snapshot.ToAgent,
		"summary":       snapshot.Summary,
		"artifact_refs": snapshot.ArtifactRefs,
	}
}

func cliContextSummary(result *clirunner.Result) string {
	if result == nil {
		return "No CLI result was produced."
	}
	if result.ContextSnapshot != nil && strings.TrimSpace(result.ContextSnapshot.Summary) != "" {
		return sharedcontext.RedactText(result.ContextSnapshot.Summary)
	}
	if strings.TrimSpace(result.Stdout) != "" {
		return oneLine(sharedcontext.RedactText(result.Stdout), 1000)
	}
	if result.Error != "" {
		return sharedcontext.RedactText(result.Error)
	}
	if len(result.ChangedFiles) > 0 {
		return "Changed files: " + strings.Join(result.ChangedFiles, ", ")
	}
	return "CLI agent completed without stdout."
}

func cliContextDecisions(result *clirunner.Result) []sharedcontext.Note {
	if result == nil || result.ContextSnapshot == nil {
		return nil
	}
	decisions := make([]sharedcontext.Note, 0, len(result.ContextSnapshot.Decisions))
	for _, decision := range result.ContextSnapshot.Decisions {
		decisions = append(decisions, sharedcontext.Note{
			Title: sharedcontext.RedactText(decision.Title),
			Body:  sharedcontext.RedactText(decision.Body),
		})
	}
	return decisions
}

func cliOpenIssues(result *clirunner.Result) []string {
	if result == nil {
		return nil
	}
	if result.ContextSnapshot != nil && len(result.ContextSnapshot.OpenIssues) > 0 {
		openIssues := make([]string, 0, len(result.ContextSnapshot.OpenIssues))
		for _, issue := range result.ContextSnapshot.OpenIssues {
			openIssues = append(openIssues, sharedcontext.RedactText(issue))
		}
		return openIssues
	}
	if result.Status == clirunner.StatusCompleted {
		return nil
	}
	if result.Error == "" {
		return []string{"Upstream CLI agent failed without a structured error."}
	}
	return []string{sharedcontext.RedactText(result.Error)}
}

func cliRecommendations(toAgent string, result *clirunner.Result) []string {
	if result == nil {
		return nil
	}
	if result.ContextSnapshot != nil && len(result.ContextSnapshot.Recommendations) > 0 {
		recommendations := make([]string, 0, len(result.ContextSnapshot.Recommendations))
		for _, recommendation := range result.ContextSnapshot.Recommendations {
			recommendations = append(recommendations, sharedcontext.RedactText(recommendation))
		}
		return recommendations
	}
	if toAgent == "" || result.Status != clirunner.StatusCompleted {
		return nil
	}
	return []string{"Use the upstream summary and artifacts as the handoff context."}
}

func cliArtifactRefs(result *clirunner.Result) []string {
	if result == nil {
		return nil
	}
	refs := []string{}
	if result.ContextSnapshot != nil {
		for _, ref := range result.ContextSnapshot.ArtifactRefs {
			if ref != "" {
				refs = append(refs, sharedcontext.RedactText(ref))
			}
		}
	}
	for _, artifact := range cliArtifacts(result) {
		if path := artifact["path"]; path != "" {
			refs = append(refs, path)
		}
	}
	return refs
}

func cliRuntimeForAgent(snapshot *graph.Snapshot, agent graph.Agent) (graph.Runtime, error) {
	runtime, ok := snapshot.IR.Runtimes[agent.Runtime]
	if !ok {
		return graph.Runtime{}, fmt.Errorf("agent %q references missing compiled runtime %q", agent.ID, agent.Runtime)
	}
	if runtime.Kind != agentspec.RuntimeKindCLIAgent {
		return graph.Runtime{}, fmt.Errorf("agent %q uses runtime %q with kind %q; only cli_agent external runtimes are executable in Gate 5 handoff execution", agent.ID, runtime.ID, runtime.Kind)
	}
	return runtime, nil
}

func outgoingEdges(snapshot *graph.Snapshot, agentID string) []graph.Edge {
	var outgoing []graph.Edge
	for _, edge := range snapshot.IR.Edges {
		if edge.From == agentID {
			outgoing = append(outgoing, edge)
		}
	}
	return outgoing
}

func handoffRunStatus(first *clirunner.Result, second *clirunner.Result) string {
	if first == nil {
		return clirunner.StatusFailed
	}
	if first.Status != clirunner.StatusCompleted {
		return first.Status
	}
	if second == nil {
		return clirunner.StatusFailed
	}
	return second.Status
}

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

func displayOutput(value string, max int) string {
	return oneLine(sharedcontext.RedactText(value), max)
}
