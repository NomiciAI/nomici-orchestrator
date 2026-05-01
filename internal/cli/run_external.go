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
