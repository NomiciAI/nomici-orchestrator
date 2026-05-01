package cli

import (
	"context"
	"fmt"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/clirunner"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/policy"
	"github.com/NomiciAI/nomici-orchestrator/internal/sharedcontext"
	tracepkg "github.com/NomiciAI/nomici-orchestrator/internal/trace"
)

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
