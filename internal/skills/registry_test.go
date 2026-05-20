package skills

import "testing"

func TestBuiltInSkillsAreInspectable(t *testing.T) {
	definitions := List("missing.yaml")
	if len(definitions) < 4 {
		t.Fatalf("expected built-in skills, got %d", len(definitions))
	}
	coding, err := Get("missing.yaml", "coding")
	if err != nil {
		t.Fatalf("get coding skill: %v", err)
	}
	if coding.Risk != "high" || coding.Briefing == "" {
		t.Fatalf("unexpected coding skill: %+v", coding)
	}
	briefings := Briefings("missing.yaml", []string{"coding", "coding", "missing"})
	if len(briefings) != 1 {
		t.Fatalf("expected one de-duplicated briefing, got %+v", briefings)
	}
}
