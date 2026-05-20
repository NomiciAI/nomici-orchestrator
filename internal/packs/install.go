package packs

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"gopkg.in/yaml.v3"
)

func InstallDeveloperTeam(ctx context.Context, options InstallOptions) (*InstallResult, error) {
	if options.ConfigPath == "" {
		options.ConfigPath = "nomici.yaml"
	}
	if options.DBPath == "" {
		options.DBPath = store.DefaultDBPath
	}

	manifest := DeveloperTeamManifest()
	profile, err := selectProviderProfile(ctx, options.DBPath, options.ModelID)
	if err != nil {
		return nil, err
	}

	spec, created, err := loadOrCreateSpec(options.ConfigPath)
	if err != nil {
		return nil, err
	}
	if spec.Models == nil {
		spec.Models = map[string]agentspec.Model{}
	}
	if spec.Agents == nil {
		spec.Agents = map[string]agentspec.Agent{}
	}
	if spec.Extensions == nil {
		spec.Extensions = map[string]any{}
	}

	modelID := profile.ID
	if existing, ok := spec.Models[modelID]; ok && !options.Force {
		if existing.Kind != providers.NormalizeKind(profile.Kind) || existing.Model != profile.Model {
			return nil, fmt.Errorf("model %q already exists in %s; use --model with a different provider profile or --force to overwrite", modelID, options.ConfigPath)
		}
	}
	spec.Models[modelID] = agentspec.Model{
		Kind:          providers.NormalizeKind(profile.Kind),
		BaseURL:       profile.BaseURL,
		APIKeyEnv:     profile.APIKeyEnv,
		Model:         profile.Model,
		Capabilities:  profileCapabilities(profile),
		ContextWindow: profile.ContextWindow,
	}

	if err := addAgent(spec, "product_pm", agentspec.Agent{
		Kind:         agentspec.AgentKindGateway,
		Model:        modelID,
		Role:         "Act as the coordinator for a small developer team. Clarify the task, decide which specialist role should own each step, and keep the run pointed at a concrete deliverable.",
		Instructions: "Keep the plan actionable. Call out risks, assumptions, test strategy, and what planner, researcher, coder, and reporter roles should do next. Do not pretend subagents executed unless the trace shows that they did.",
		Subagents:    []string{"planner", "researcher", "coder", "reporter"},
	}, options.Force); err != nil {
		return nil, err
	}
	if err := addAgent(spec, "planner", agentspec.Agent{
		Kind:         agentspec.AgentKindModel,
		Model:        modelID,
		Role:         "Break the user goal into a bounded plan with phases, acceptance criteria, dependencies, and open questions.",
		Instructions: "Prefer short plans with explicit sequencing. Separate what can run now from work blocked on tools, data, or approvals.",
	}, options.Force); err != nil {
		return nil, err
	}
	if err := addAgent(spec, "researcher", agentspec.Agent{
		Kind:         agentspec.AgentKindModel,
		Model:        modelID,
		Role:         "Gather and verify external or project information needed for the task.",
		Instructions: "Summarize sources, confidence, contradictions, and follow-up checks. Do not invent citations or claim live search if no search tool was available.",
	}, options.Force); err != nil {
		return nil, err
	}
	if err := addAgent(spec, "coder", agentspec.Agent{
		Kind:         agentspec.AgentKindModel,
		Model:        modelID,
		Role:         "Analyze implementation options and produce code-oriented guidance for the task.",
		Instructions: "Focus on boundaries, data model, testability, security implications, and verification strategy.",
	}, options.Force); err != nil {
		return nil, err
	}
	if err := addAgent(spec, "reporter", agentspec.Agent{
		Kind:         agentspec.AgentKindModel,
		Model:        modelID,
		Role:         "Turn completed work and evidence into a concise final report or handoff.",
		Instructions: "Report what changed, what was verified, residual risks, and the next concrete step. Keep evidence tied to trace, artifacts, or context snapshots.",
	}, options.Force); err != nil {
		return nil, err
	}
	if err := addAgent(spec, "architect", agentspec.Agent{
		Kind:         agentspec.AgentKindModel,
		Model:        modelID,
		Role:         "Review product intent and produce software architecture guidance for implementation work.",
		Instructions: "Focus on boundaries, data model, testability, and security implications.",
	}, options.Force); err != nil {
		return nil, err
	}

	installedAt := time.Now().UTC()
	packs := extensionPacks(spec)
	packs[manifest.ID] = map[string]any{
		"version":      manifest.Version,
		"kind":         manifest.Kind,
		"trust":        "official",
		"installed_at": installedAt.Format(time.RFC3339Nano),
		"entrypoints":  manifest.Agents.Entrypoints,
		"roles":        manifest.Roles,
	}
	spec.Extensions["packs"] = packs

	if err := writeSpec(options.ConfigPath, spec); err != nil {
		return nil, err
	}
	if err := savePackInstallation(ctx, options.DBPath, manifest, options.ConfigPath, installedAt); err != nil {
		return nil, err
	}

	return &InstallResult{
		PackID:      manifest.ID,
		ConfigPath:  options.ConfigPath,
		ModelID:     modelID,
		Created:     created,
		Updated:     true,
		InstalledAt: installedAt,
	}, nil
}

func selectProviderProfile(ctx context.Context, dbPath string, requested string) (*providers.Profile, error) {
	db, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		return nil, err
	}

	providerStore := providers.NewStore(db)
	if requested != "" {
		profile, err := providerStore.Get(ctx, requested)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("model profile %q was not found; run `nomici model list`", requested)
			}
			return nil, err
		}
		return profile, nil
	}

	profiles, err := providerStore.List(ctx)
	if err != nil {
		return nil, err
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("developer-team requires one configured model profile. Remediation: run `nomici model setup --kind openai_compatible --name gpt --model <model> --api-key-env OPENAI_API_KEY`")
	}
	return profiles[0], nil
}

func loadOrCreateSpec(configPath string) (*agentspec.Spec, bool, error) {
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			projectName := filepath.Base(filepath.Dir(configPath))
			if projectName == "." || projectName == string(filepath.Separator) || projectName == "" {
				projectName = "nomici-project"
			}
			return &agentspec.Spec{
				Version: "0.1",
				Project: agentspec.Project{
					Name:        projectName,
					Description: "Created by Nomici developer-team pack.",
				},
			}, true, nil
		}
		return nil, false, fmt.Errorf("stat AgentSpec: %w", err)
	}
	loaded, err := agentspec.LoadFile(configPath)
	if err != nil {
		return nil, false, err
	}
	if errors := agentspec.Validate(loaded); len(errors) > 0 {
		return nil, false, fmt.Errorf("existing AgentSpec is invalid; run `nomici spec validate --config %s` before installing packs", configPath)
	}
	return loaded.Spec, false, nil
}

func addAgent(spec *agentspec.Spec, id string, agent agentspec.Agent, force bool) error {
	if existing, ok := spec.Agents[id]; ok && !force {
		if existing.Kind != agent.Kind || existing.Model != agent.Model {
			return fmt.Errorf("agent %q already exists; use --force to overwrite pack-managed agents", id)
		}
		return nil
	}
	spec.Agents[id] = agent
	return nil
}

func writeSpec(configPath string, spec *agentspec.Spec) error {
	if dir := filepath.Dir(configPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create config directory: %w", err)
		}
	}
	payload, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshal AgentSpec: %w", err)
	}
	return os.WriteFile(configPath, payload, 0o644)
}

func savePackInstallation(ctx context.Context, dbPath string, manifest Manifest, configPath string, installedAt time.Time) error {
	db, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		return err
	}
	return NewStore(db).SaveInstallation(ctx, &Installation{
		PackID:      manifest.ID,
		Version:     manifest.Version,
		Kind:        manifest.Kind,
		Trust:       manifest.Trust.Level,
		ConfigPath:  configPath,
		Entrypoints: manifest.Agents.Entrypoints,
		InstalledAt: installedAt,
		UpdatedAt:   installedAt,
	})
}

func profileCapabilities(profile *providers.Profile) []string {
	capabilities := []string{}
	for key, value := range profile.Capabilities {
		if value == "true" {
			capabilities = append(capabilities, key)
		}
	}
	return capabilities
}

func extensionPacks(spec *agentspec.Spec) map[string]any {
	existing, ok := spec.Extensions["packs"].(map[string]any)
	if ok {
		return existing
	}
	converted := map[string]any{}
	if raw, ok := spec.Extensions["packs"].(map[any]any); ok {
		for key, value := range raw {
			if text, ok := key.(string); ok {
				converted[text] = value
			}
		}
	}
	return converted
}
