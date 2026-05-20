package gateway

import (
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/orchestration"
)

func TestSemanticRouteConfidenceGate(t *testing.T) {
	fallback := orchestration.RouteDecision{Mode: orchestration.ModeWorkspaceRun}
	if semanticRouteAccepted(orchestration.RouteDecision{
		Mode:       orchestration.ModeDirectReply,
		Confidence: 0.42,
	}, fallback) {
		t.Fatalf("low-confidence direct reply should not override workspace fallback")
	}
	if !semanticRouteAccepted(orchestration.RouteDecision{
		Mode:          orchestration.ModeClarify,
		Confidence:    0.45,
		MissingInputs: []string{"target_file"},
		Clarification: "Which file should Nomici modify?",
	}, fallback) {
		t.Fatalf("low-confidence clarification with missing inputs should be accepted")
	}
	if !semanticRouteAccepted(orchestration.RouteDecision{
		Mode:       orchestration.ModeWorkspaceRun,
		Confidence: 0.8,
	}, fallback) {
		t.Fatalf("high-confidence workspace route should be accepted")
	}
}
