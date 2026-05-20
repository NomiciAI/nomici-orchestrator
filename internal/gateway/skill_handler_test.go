package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/skills"
)

func TestSkillCreateAndToggleEndpoints(t *testing.T) {
	_, router := newRunTestRouter(t)

	createBody := `{
		"id": "repo_inspection",
		"name": "Repo Inspection",
		"description": "Inspect project structure and summarize risks.",
		"triggers": ["inspect", "audit"],
		"required_tools": ["list_files", "read_file"],
		"risk": "medium",
		"compatibility": "local",
		"briefing": "Start by reading the project layout before proposing changes."
	}`
	createResponse := httptest.NewRecorder()
	router.ServeHTTP(
		createResponse,
		httptest.NewRequest(http.MethodPost, "/api/skills", bytes.NewBufferString(createBody)),
	)
	if createResponse.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", createResponse.Code, createResponse.Body.String())
	}
	created := decodeSkillEnvelope(t, createResponse.Body.Bytes())
	if created.ID != "repo_inspection" || !created.Enabled {
		t.Fatalf("expected created skill to be enabled, got %+v", created)
	}
	if created.Source != "project" {
		t.Fatalf("expected project skill source, got %q", created.Source)
	}

	disableResponse := httptest.NewRecorder()
	router.ServeHTTP(
		disableResponse,
		httptest.NewRequest(http.MethodPatch, "/api/skills/repo_inspection", bytes.NewBufferString(`{"enabled": false}`)),
	)
	if disableResponse.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", disableResponse.Code, disableResponse.Body.String())
	}
	disabled := decodeSkillEnvelope(t, disableResponse.Body.Bytes())
	if disabled.Enabled || !disabled.Disabled {
		t.Fatalf("expected disabled skill, got %+v", disabled)
	}

	detailResponse := httptest.NewRecorder()
	router.ServeHTTP(detailResponse, httptest.NewRequest(http.MethodGet, "/api/skills/repo_inspection", nil))
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", detailResponse.Code, detailResponse.Body.String())
	}
	detail := decodeSkillEnvelope(t, detailResponse.Body.Bytes())
	if detail.Enabled {
		t.Fatalf("expected detail to preserve disabled state, got %+v", detail)
	}
}

func TestSkillPatchRequiresEnabled(t *testing.T) {
	_, router := newRunTestRouter(t)
	response := httptest.NewRecorder()
	router.ServeHTTP(
		response,
		httptest.NewRequest(http.MethodPatch, "/api/skills/planning", bytes.NewBufferString(`{}`)),
	)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "enabled is required") {
		t.Fatalf("expected clear validation error, got %s", response.Body.String())
	}
}

func decodeSkillEnvelope(t *testing.T, payload []byte) skills.Definition {
	t.Helper()
	var envelope struct {
		Data skills.Definition `json:"data"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("decode skill envelope: %v", err)
	}
	return envelope.Data
}
