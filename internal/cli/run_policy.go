package cli

import (
	"context"
	"fmt"

	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
	"github.com/NomiciAI/nomici-orchestrator/internal/policy"
	tracepkg "github.com/NomiciAI/nomici-orchestrator/internal/trace"
)

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
