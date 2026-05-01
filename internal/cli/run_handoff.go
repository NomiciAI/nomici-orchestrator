package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/clirunner"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
	"github.com/NomiciAI/nomici-orchestrator/internal/policy"
	"github.com/NomiciAI/nomici-orchestrator/internal/sharedcontext"
	tracepkg "github.com/NomiciAI/nomici-orchestrator/internal/trace"
	"github.com/spf13/cobra"
)

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
	if err := appendHandoffCreated(ctx, traceStore, runID, fromAgent, toAgent, fromRuntime, edge, handoffSnapshot); err != nil {
		return err
	}
	if err := appendHandoffContextAttached(ctx, traceStore, runID, fromAgent, toAgent, toRuntime, handoffSnapshot); err != nil {
		return err
	}

	runType := tracepkg.EventRunCompleted
	second := (*clirunner.Result)(nil)
	if first.Status == clirunner.StatusCompleted {
		second, err = acceptAndInvokeHandoff(ctx, traceStore, policyService, contextStore, snapshot, fromAgent, toAgent, toRuntime, configPath, prompt, runID, taskID, handoffSnapshot)
		if err != nil {
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
