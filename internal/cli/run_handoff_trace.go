package cli

import (
	"context"

	"github.com/NomiciAI/nomici-orchestrator/internal/clirunner"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/policy"
	"github.com/NomiciAI/nomici-orchestrator/internal/sharedcontext"
	tracepkg "github.com/NomiciAI/nomici-orchestrator/internal/trace"
)

func appendHandoffCreated(ctx context.Context, traceStore *tracepkg.Store, runID string, fromAgent graph.Agent, toAgent graph.Agent, fromRuntime graph.Runtime, edge graph.Edge, snapshot *sharedcontext.Snapshot) error {
	return traceStore.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventHandoffCreated,
		NodeID:    fromAgent.ID,
		RuntimeID: fromRuntime.ID,
		Payload: jsonPayload(map[string]any{
			"from":        fromAgent.ID,
			"to":          toAgent.ID,
			"edge_id":     edge.ID,
			"mode":        edge.Mode,
			"snapshot_id": snapshot.SnapshotID,
		}),
	})
}

func appendHandoffContextAttached(ctx context.Context, traceStore *tracepkg.Store, runID string, fromAgent graph.Agent, toAgent graph.Agent, toRuntime graph.Runtime, snapshot *sharedcontext.Snapshot) error {
	return traceStore.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventHandoffContextAttached,
		NodeID:    toAgent.ID,
		RuntimeID: toRuntime.ID,
		Payload: jsonPayload(map[string]any{
			"from":        fromAgent.ID,
			"to":          toAgent.ID,
			"snapshot_id": snapshot.SnapshotID,
		}),
	})
}

func acceptAndInvokeHandoff(ctx context.Context, traceStore *tracepkg.Store, policyService *policy.Service, contextStore *sharedcontext.Store, snapshot *graph.Snapshot, fromAgent graph.Agent, toAgent graph.Agent, toRuntime graph.Runtime, configPath string, prompt string, runID string, taskID string, handoffSnapshot *sharedcontext.Snapshot) (*clirunner.Result, error) {
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
		return nil, err
	}

	result, err := invokeExternalCLIAgent(ctx, traceStore, policyService, snapshot.ProjectID, toRuntime, toAgent, configPath, runID, taskID, graphPrompt(toAgent, prompt), handoffSnapshot)
	if err != nil {
		_ = appendExternalFailure(ctx, traceStore, runID, toAgent.ID, toRuntime.ID, err.Error())
		return nil, err
	}
	if _, err := saveCLIContextSnapshot(ctx, contextStore, traceStore, snapshot.ProjectID, runID, taskID, toAgent.ID, "", result, sharedcontext.KindRunSummary); err != nil {
		return nil, err
	}
	return result, nil
}
