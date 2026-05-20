package projectconfig

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"gopkg.in/yaml.v3"
)

var validID = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

type AgentRecord struct {
	ID           string         `json:"id"`
	Name         string         `json:"name,omitempty"`
	Description  string         `json:"description,omitempty"`
	Kind         string         `json:"kind"`
	Model        string         `json:"model,omitempty"`
	Runtime      string         `json:"runtime,omitempty"`
	Role         string         `json:"role,omitempty"`
	Instructions string         `json:"instructions,omitempty"`
	Tools        []string       `json:"tools,omitempty"`
	Skills       []string       `json:"skills,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	Triggers     []string       `json:"triggers,omitempty"`
	Capabilities map[string]any `json:"capabilities,omitempty"`
}

type OrchestrationConfig struct {
	Entrypoint       string                    `json:"entrypoint" yaml:"entrypoint,omitempty"`
	RoleOrder        []string                  `json:"role_order" yaml:"role_order,omitempty"`
	DisabledRoles    []string                  `json:"disabled_roles" yaml:"disabled_roles,omitempty"`
	PlanReviewPolicy string                    `json:"plan_review_policy" yaml:"plan_review_policy,omitempty"`
	Roles            map[string]RoleConfig     `json:"roles,omitempty" yaml:"roles,omitempty"`
	Metadata         map[string]map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type RoleConfig struct {
	Purpose        string                   `json:"purpose,omitempty" yaml:"purpose,omitempty"`
	Instructions   string                   `json:"instructions,omitempty" yaml:"instructions,omitempty"`
	OutputContract agentspec.OutputContract `json:"output_contract,omitempty" yaml:"output_contract,omitempty"`
}

func ListAgents(configPath string) ([]AgentRecord, error) {
	loaded, err := agentspec.LoadFile(configPath)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(loaded.Spec.Agents))
	for id := range loaded.Spec.Agents {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	records := make([]AgentRecord, 0, len(ids))
	for _, id := range ids {
		records = append(records, agentRecord(id, loaded.Spec.Agents[id]))
	}
	return records, nil
}

func GetAgent(configPath string, id string) (*AgentRecord, error) {
	loaded, err := agentspec.LoadFile(configPath)
	if err != nil {
		return nil, err
	}
	agent, ok := loaded.Spec.Agents[id]
	if !ok {
		return nil, fmt.Errorf("agent %q was not found", id)
	}
	record := agentRecord(id, agent)
	return &record, nil
}

func UpsertAgent(ctx context.Context, configPath string, dbPath string, record AgentRecord) (*graph.Snapshot, error) {
	if err := validateAgentRecord(record); err != nil {
		return nil, err
	}
	spec, err := loadOrCreate(configPath)
	if err != nil {
		return nil, err
	}
	if spec.Agents == nil {
		spec.Agents = map[string]agentspec.Agent{}
	}
	spec.Agents[record.ID] = agentspec.Agent{
		Name:         strings.TrimSpace(record.Name),
		Description:  strings.TrimSpace(record.Description),
		Kind:         strings.TrimSpace(record.Kind),
		Model:        strings.TrimSpace(record.Model),
		Runtime:      strings.TrimSpace(record.Runtime),
		Role:         strings.TrimSpace(record.Role),
		Instructions: strings.TrimSpace(record.Instructions),
		Tools:        cleanList(record.Tools),
		Skills:       cleanList(record.Skills),
		Tags:         cleanList(record.Tags),
		Triggers:     cleanList(record.Triggers),
		Capabilities: record.Capabilities,
	}
	if err := save(configPath, spec); err != nil {
		return nil, err
	}
	return compileAndSave(ctx, configPath, dbPath)
}

func ValidateAgent(record AgentRecord) error {
	return validateAgentRecord(record)
}

func DeleteAgent(ctx context.Context, configPath string, dbPath string, id string) (*graph.Snapshot, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	spec, err := loadOrCreate(configPath)
	if err != nil {
		return nil, err
	}
	if _, ok := spec.Agents[id]; !ok {
		return nil, fmt.Errorf("agent %q was not found", id)
	}
	delete(spec.Agents, id)
	filtered := spec.Edges[:0]
	for _, edge := range spec.Edges {
		if edge.From != id && edge.To != id {
			filtered = append(filtered, edge)
		}
	}
	spec.Edges = filtered
	if err := save(configPath, spec); err != nil {
		return nil, err
	}
	return compileAndSave(ctx, configPath, dbPath)
}

func GetOrchestration(configPath string) (OrchestrationConfig, error) {
	loaded, err := agentspec.LoadFile(configPath)
	if err != nil {
		return OrchestrationConfig{}, err
	}
	return orchestrationConfig(loaded.Spec), nil
}

func SaveOrchestration(ctx context.Context, configPath string, dbPath string, config OrchestrationConfig) (*graph.Snapshot, error) {
	spec, err := loadOrCreate(configPath)
	if err != nil {
		return nil, err
	}
	if spec.Extensions == nil {
		spec.Extensions = map[string]any{}
	}
	normalized := normalizeOrchestration(config)
	spec.Extensions["orchestration"] = normalized
	if err := applyRoleOverrides(spec, normalized); err != nil {
		return nil, err
	}
	if err := save(configPath, spec); err != nil {
		return nil, err
	}
	return compileAndSave(ctx, configPath, dbPath)
}

func compileAndSave(ctx context.Context, configPath string, dbPath string) (*graph.Snapshot, error) {
	loaded, err := agentspec.LoadFileWithLocal(configPath)
	if err != nil {
		return nil, err
	}
	snapshot, errors := graph.Compile(loaded)
	if len(errors) > 0 {
		return nil, fmt.Errorf("AgentGraph validation failed with %d error(s)", len(errors))
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		return nil, err
	}
	if err := graph.NewStore(db).Save(ctx, snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func loadOrCreate(configPath string) (*agentspec.Spec, error) {
	if _, err := os.Stat(configPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		return &agentspec.Spec{
			Version: "0.1",
			Project: agentspec.Project{
				Name:        defaultProjectName(configPath),
				Description: "Created by Nomici.",
			},
		}, nil
	}
	loaded, err := agentspec.LoadFile(configPath)
	if err != nil {
		return nil, err
	}
	return loaded.Spec, nil
}

func save(configPath string, spec *agentspec.Spec) error {
	if dir := filepath.Dir(configPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	payload, err := yaml.Marshal(spec)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, payload, 0o644)
}

func validateAgentRecord(record AgentRecord) error {
	if !validID.MatchString(strings.TrimSpace(record.ID)) {
		return fmt.Errorf("agent id must start with a letter and contain only letters, numbers, underscores, or dashes")
	}
	switch record.Kind {
	case agentspec.AgentKindGateway, agentspec.AgentKindModel:
		if strings.TrimSpace(record.Model) == "" {
			return fmt.Errorf("model-backed agents require a model reference")
		}
	case agentspec.AgentKindExternal:
		if strings.TrimSpace(record.Runtime) == "" {
			return fmt.Errorf("external agents require a runtime reference")
		}
	case agentspec.AgentKindTool:
	default:
		return fmt.Errorf("unsupported agent kind %q", record.Kind)
	}
	return nil
}

func agentRecord(id string, agent agentspec.Agent) AgentRecord {
	return AgentRecord{
		ID:           id,
		Name:         agent.Name,
		Description:  agent.Description,
		Kind:         agent.Kind,
		Model:        agent.Model,
		Runtime:      agent.Runtime,
		Role:         agent.Role,
		Instructions: agent.Instructions,
		Tools:        agent.Tools,
		Skills:       agent.Skills,
		Tags:         agent.Tags,
		Triggers:     agent.Triggers,
		Capabilities: agent.Capabilities,
	}
}

func orchestrationConfig(spec *agentspec.Spec) OrchestrationConfig {
	if spec == nil || spec.Extensions == nil {
		return OrchestrationConfig{}
	}
	raw, ok := spec.Extensions["orchestration"]
	if !ok {
		return OrchestrationConfig{}
	}
	payload, _ := yaml.Marshal(raw)
	var config OrchestrationConfig
	_ = yaml.Unmarshal(payload, &config)
	return config
}

func normalizeOrchestration(config OrchestrationConfig) OrchestrationConfig {
	config.Entrypoint = strings.TrimSpace(config.Entrypoint)
	config.RoleOrder = cleanList(config.RoleOrder)
	config.DisabledRoles = cleanList(config.DisabledRoles)
	if config.PlanReviewPolicy == "" {
		config.PlanReviewPolicy = "auto"
	}
	if config.Roles == nil {
		config.Roles = map[string]RoleConfig{}
	}
	if config.Metadata == nil {
		config.Metadata = map[string]map[string]any{}
	}
	return config
}

func applyRoleOverrides(spec *agentspec.Spec, config OrchestrationConfig) error {
	for roleID, role := range config.Roles {
		agent, ok := spec.Agents[roleID]
		if !ok {
			return fmt.Errorf("orchestration role %q does not match an agent", roleID)
		}
		if role.Purpose != "" {
			agent.Role = role.Purpose
		}
		if role.Instructions != "" {
			agent.Instructions = role.Instructions
		}
		spec.Agents[roleID] = agent
	}
	return nil
}

func cleanList(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func defaultProjectName(configPath string) string {
	dir := filepath.Dir(configPath)
	if dir == "." || dir == "" {
		if cwd, err := os.Getwd(); err == nil {
			dir = cwd
		}
	}
	name := filepath.Base(dir)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "nomici-project"
	}
	return strings.ReplaceAll(strings.ToLower(name), " ", "-") + "-" + time.Now().UTC().Format("20060102")
}
