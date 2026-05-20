package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBriefingsLoadsProjectSkillFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "skill-notes.md"), []byte("Use the project-specific review checklist."), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "nomici.yaml")
	payload := []byte(`version: "0.1"
project:
  name: skill-test
extensions:
  skills:
    - id: project-review
      name: Project Review
      briefing: Check local conventions.
      files:
        - skill-notes.md
`)
	if err := os.WriteFile(configPath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	briefings := Briefings(configPath, []string{"project-review"})
	if len(briefings) != 1 || !strings.Contains(briefings[0], "Use the project-specific review checklist.") {
		t.Fatalf("expected file-backed briefing, got %#v", briefings)
	}
}
