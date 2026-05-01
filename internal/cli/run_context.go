package cli

import (
	"context"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/clirunner"
	"github.com/NomiciAI/nomici-orchestrator/internal/sharedcontext"
	tracepkg "github.com/NomiciAI/nomici-orchestrator/internal/trace"
)

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
