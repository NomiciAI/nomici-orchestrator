package skills

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"gopkg.in/yaml.v3"
)

type Definition struct {
	ID            string   `json:"id" yaml:"id"`
	Name          string   `json:"name" yaml:"name"`
	Description   string   `json:"description" yaml:"description"`
	Triggers      []string `json:"triggers,omitempty" yaml:"triggers,omitempty"`
	Files         []string `json:"files,omitempty" yaml:"files,omitempty"`
	RequiredTools []string `json:"required_tools,omitempty" yaml:"required_tools,omitempty"`
	Risk          string   `json:"risk,omitempty" yaml:"risk,omitempty"`
	Compatibility string   `json:"compatibility,omitempty" yaml:"compatibility,omitempty"`
	Briefing      string   `json:"briefing,omitempty" yaml:"briefing,omitempty"`
	Source        string   `json:"source,omitempty" yaml:"source,omitempty"`
}

func List(configPath string) []Definition {
	definitions := Builtins()
	definitions = append(definitions, extensionSkills(configPath)...)
	byID := map[string]Definition{}
	for _, definition := range definitions {
		if strings.TrimSpace(definition.ID) == "" {
			continue
		}
		byID[definition.ID] = normalize(definition)
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]Definition, 0, len(ids))
	for _, id := range ids {
		result = append(result, byID[id])
	}
	return result
}

func Get(configPath string, id string) (Definition, error) {
	for _, definition := range List(configPath) {
		if definition.ID == id {
			return definition, nil
		}
	}
	return Definition{}, fmt.Errorf("skill %q was not found", id)
}

func Briefings(configPath string, ids []string) []string {
	briefings := []string{}
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		definition, err := Get(configPath, id)
		if err != nil || definition.Briefing == "" {
			continue
		}
		briefings = append(briefings, definition.Name+": "+definition.Briefing)
	}
	return briefings
}

func Builtins() []Definition {
	return []Definition{
		{
			ID:            "planning",
			Name:          "Planning",
			Description:   "Break a goal into bounded phases, dependencies, risks, and acceptance criteria.",
			Triggers:      []string{"plan", "roadmap", "scope"},
			Risk:          "low",
			Compatibility: "local",
			Briefing:      "Produce an executable plan with scope, order, risks, and acceptance criteria. Surface missing inputs before execution.",
			Source:        "builtin",
		},
		{
			ID:            "research",
			Name:          "Research",
			Description:   "Gather and summarize project or web facts with confidence and contradictions.",
			Triggers:      []string{"research", "compare", "investigate"},
			RequiredTools: []string{"search", "fetch", "read_file"},
			Risk:          "medium",
			Compatibility: "local",
			Briefing:      "Separate verified facts from assumptions. Include source context, confidence, contradictions, and follow-up checks.",
			Source:        "builtin",
		},
		{
			ID:            "coding",
			Name:          "Coding",
			Description:   "Implement changes through mediated workspace tools and verification.",
			Triggers:      []string{"implement", "fix", "refactor", "test"},
			RequiredTools: []string{"list_files", "read_file", "write_file", "replace_file", "bash"},
			Risk:          "high",
			Compatibility: "local",
			Briefing:      "Use mediated tools for files and commands. Keep edits scoped, verify behavior, and report changed surfaces and residual risk.",
			Source:        "builtin",
		},
		{
			ID:            "reporting",
			Name:          "Reporting",
			Description:   "Produce final user-facing summaries tied to trace, artifacts, and verification.",
			Triggers:      []string{"report", "summary", "deliver"},
			Risk:          "low",
			Compatibility: "local",
			Briefing:      "Summarize what changed, what was verified, artifacts produced, and remaining risks without claiming unexecuted work.",
			Source:        "builtin",
		},
	}
}

func extensionSkills(configPath string) []Definition {
	if strings.TrimSpace(configPath) == "" {
		configPath = "nomici.yaml"
	}
	loaded, err := agentspec.LoadFileWithLocal(configPath)
	if err != nil || loaded.Spec == nil || loaded.Spec.Extensions == nil {
		return nil
	}
	raw, ok := loaded.Spec.Extensions["skills"]
	if !ok {
		return nil
	}
	payload, err := yaml.Marshal(raw)
	if err != nil {
		return nil
	}
	var definitions []Definition
	if err := yaml.Unmarshal(payload, &definitions); err == nil && len(definitions) > 0 {
		for index := range definitions {
			definitions[index].Source = "project"
		}
		return definitions
	}
	var byID map[string]Definition
	if err := yaml.Unmarshal(payload, &byID); err != nil {
		return nil
	}
	definitions = make([]Definition, 0, len(byID))
	for id, definition := range byID {
		if definition.ID == "" {
			definition.ID = id
		}
		definition.Source = "project"
		definitions = append(definitions, definition)
	}
	return definitions
}

func normalize(definition Definition) Definition {
	definition.ID = strings.TrimSpace(definition.ID)
	if definition.Name == "" {
		definition.Name = definition.ID
	}
	if definition.Risk == "" {
		definition.Risk = "low"
	}
	if definition.Compatibility == "" {
		definition.Compatibility = "local"
	}
	if definition.Source == "" {
		definition.Source = "project"
	}
	return definition
}

func Marshal(definition Definition) ([]byte, error) {
	return json.MarshalIndent(definition, "", "  ")
}
