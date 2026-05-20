package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/packs"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	setupPackDeveloperTeam = "developer-team"
	setupPackNone          = "none"

	sandboxModeContainer = "container"
	sandboxModeLocal     = "local"
	sandboxModeNone      = "none"
)

type setupOptions struct {
	configPath      string
	dbPath          string
	provider        string
	profileName     string
	model           string
	baseURL         string
	apiKeyEnv       string
	apiKeyValue     string
	packID          string
	webSearch       string
	webSearchKeyEnv string
	webFetch        string
	webFetchKeyEnv  string
	sandboxMode     string
	enableBash      bool
	enableFileWrite bool
	yes             bool
	force           bool
}

type setupChoice struct {
	ID          string
	Label       string
	Description string
}

func newSetupCommand() *cobra.Command {
	options := setupOptions{
		configPath:      "nomici.yaml",
		dbPath:          store.DefaultDBPath,
		packID:          setupPackDeveloperTeam,
		webSearch:       "duckduckgo",
		webFetch:        "jina_reader",
		sandboxMode:     sandboxModeLocal,
		enableBash:      false,
		enableFileWrite: true,
	}
	command := &cobra.Command{
		Use:   "setup",
		Short: "Configure a usable Nomici workspace",
		Long:  "Configure a usable Nomici workspace by creating a model profile, configuring web tools, installing a starter pack, and writing sandbox intent into nomici.yaml.",
		RunE: func(command *cobra.Command, args []string) error {
			return runSetup(command, options)
		},
	}
	command.Flags().StringVar(&options.configPath, "config", options.configPath, "AgentSpec config path")
	command.Flags().StringVar(&options.dbPath, "db-path", options.dbPath, "SQLite database path")
	command.Flags().StringVar(&options.provider, "provider", "", "Provider to configure: openai, anthropic, gemini, deepseek, openrouter, vllm, ollama, codex-cli, claude-code, or other-openai-compatible")
	command.Flags().StringVar(&options.profileName, "name", "", "Model profile name")
	command.Flags().StringVar(&options.model, "model", "", "Provider model name")
	command.Flags().StringVar(&options.baseURL, "base-url", "", "Provider base URL")
	command.Flags().StringVar(&options.apiKeyEnv, "api-key-env", "", "Environment variable containing the API key")
	command.Flags().StringVar(&options.packID, "pack", options.packID, "Starter pack to install: developer-team or none")
	command.Flags().StringVar(&options.webSearch, "web-search", options.webSearch, "Web search provider: duckduckgo, brave, tavily, or none")
	command.Flags().StringVar(&options.webSearchKeyEnv, "web-search-api-key-env", "", "Environment variable containing the web search API key")
	command.Flags().StringVar(&options.webFetch, "web-fetch", options.webFetch, "Web fetch provider: jina-reader, direct-http, or none")
	command.Flags().StringVar(&options.webFetchKeyEnv, "web-fetch-api-key-env", "", "Environment variable containing the web fetch API key")
	command.Flags().StringVar(&options.sandboxMode, "sandbox", options.sandboxMode, "Sandbox mode: local, container, or none")
	command.Flags().BoolVar(&options.enableBash, "enable-bash", options.enableBash, "Allow bash command execution in sandbox policy intent")
	command.Flags().BoolVar(&options.enableFileWrite, "enable-file-write", options.enableFileWrite, "Allow file write tools in sandbox policy intent")
	command.Flags().BoolVar(&options.yes, "yes", false, "Accept defaults for omitted optional setup choices")
	command.Flags().BoolVar(&options.force, "force", false, "Overwrite pack-managed model or agent definitions")
	return command
}

func runSetup(command *cobra.Command, options setupOptions) error {
	in := bufio.NewReader(command.InOrStdin())
	out := command.OutOrStdout()
	interactive := !options.yes

	fmt.Fprintln(out, "Welcome to Nomici Setup.")
	fmt.Fprintln(out, "This wizard configures the first usable model, web tools, starter pack, and sandbox policy for this workspace.")

	if err := collectProviderOptions(in, out, &options, interactive); err != nil {
		return err
	}
	if err := collectWebToolOptions(in, out, &options, interactive); err != nil {
		return err
	}
	if err := collectPackOptions(in, out, &options, interactive); err != nil {
		return err
	}
	if err := collectSandboxOptions(in, out, &options, interactive); err != nil {
		return err
	}

	profile, err := saveSetupProfile(command.Context(), options)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "\nStep 4/4 - Writing configuration")
	fmt.Fprintf(out, "\n  ✓ Model profile saved: %s (%s / %s)\n", profile.ID, profile.Kind, profile.Model)
	if options.apiKeyValue != "" && profile.APIKeyEnv != "" {
		if err := writeLocalSecret(options.configPath, profile.APIKeyEnv, options.apiKeyValue); err != nil {
			return err
		}
		fmt.Fprintf(out, "  ✓ Local secret saved: %s (%s)\n", localSecretPath(options.configPath), profile.APIKeyEnv)
	}

	if options.packID == setupPackDeveloperTeam {
		result, err := packs.InstallDeveloperTeam(command.Context(), packs.InstallOptions{
			ConfigPath: options.configPath,
			DBPath:     options.dbPath,
			ModelID:    profile.ID,
			Force:      options.force,
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "  ✓ Pack installed: %s -> %s\n", result.PackID, result.ConfigPath)
	} else if err := ensureSetupSpec(options.configPath); err != nil {
		return err
	}

	if err := writeSandboxConfig(options.configPath, sandboxConfigFromOptions(options)); err != nil {
		return err
	}
	fmt.Fprintf(out, "  ✓ Sandbox configured: %s\n", options.sandboxMode)
	if err := writeWebToolConfig(options.configPath, webToolConfigFromOptions(options)); err != nil {
		return err
	}
	fmt.Fprintf(out, "  ✓ Web tools configured: search=%s fetch=%s\n", options.webSearch, options.webFetch)

	snapshot, err := compileGraphSnapshot(command.Context(), options.configPath, options.dbPath)
	if err != nil {
		return err
	}
	if snapshot != nil {
		fmt.Fprintf(out, "  ✓ Graph snapshot saved: %s\n", snapshot.SnapshotID)
	}

	printSetupSummary(out, options, profile)
	return nil
}

func collectProviderOptions(in *bufio.Reader, out io.Writer, options *setupOptions, interactive bool) error {
	fmt.Fprintln(out, "\nStep 1/4 - Choose your LLM provider")
	providerChoices := setupProviderChoices()
	if strings.TrimSpace(options.provider) == "" {
		if !interactive {
			return fmt.Errorf("--provider is required when --yes is used")
		}
		choice, err := promptChoice(in, out, "Provider", providerChoices, 0)
		if err != nil {
			return err
		}
		options.provider = choice.ID
	}
	options.provider = providers.NormalizeProviderID(options.provider)
	definition, ok := providers.GetProviderDefinition(options.provider)
	if !ok {
		return fmt.Errorf("unsupported provider %q; run `nomici provider list` to see available providers", options.provider)
	}
	if definition.Local && !definition.Available {
		return fmt.Errorf("%s provider is not ready: %s", definition.ID, definition.AvailabilityMessage)
	}

	if options.baseURL == "" {
		options.baseURL = definition.DefaultBaseURL
	}
	if interactive && definition.RequiresBaseURL {
		value, err := promptString(in, out, "Base URL", options.baseURL, false)
		if err != nil {
			return err
		}
		options.baseURL = value
	}

	if options.apiKeyEnv == "" && definition.AuthMode == providers.AuthModeAPIKeyEnv {
		options.apiKeyEnv = definition.DefaultAPIKeyEnv
	}
	if interactive && definition.AuthMode == providers.AuthModeAPIKeyEnv {
		value, rawSecret, err := promptAPIKeyEnvOrSecret(in, out, "API key env var or key", options.apiKeyEnv, options.configPath)
		if err != nil {
			return err
		}
		options.apiKeyEnv = value
		if rawSecret != "" {
			options.apiKeyValue = rawSecret
			if err := os.Setenv(options.apiKeyEnv, rawSecret); err != nil {
				return fmt.Errorf("set local setup secret env: %w", err)
			}
		}
	}
	if definition.AuthMode == providers.AuthModeAPIKeyEnv {
		if err := validateAPIKeyEnv(options.apiKeyEnv); err != nil {
			return err
		}
	} else {
		options.apiKeyEnv = ""
	}

	if strings.TrimSpace(options.model) == "" {
		if !interactive {
			return fmt.Errorf("--model is required when --yes is used")
		}
		value, err := promptProviderModel(in, out, definition, options)
		if err != nil {
			return err
		}
		options.model = value
	}

	if options.profileName == "" {
		options.profileName = definition.ID + " " + options.model
	}
	if definition.Local && options.profileName == "" {
		options.profileName = definition.ID
	}
	return nil
}

func setupProviderChoices() []setupChoice {
	var choices []setupChoice
	for _, provider := range providers.ProviderCatalog() {
		description := provider.Description
		if provider.Local {
			if provider.Available {
				description += "; ready: " + provider.AvailabilityMessage
			} else {
				description += "; not ready: " + provider.AvailabilityMessage
			}
		}
		choices = append(choices, setupChoice{
			ID:          provider.ID,
			Label:       provider.Name,
			Description: description,
		})
	}
	return choices
}

func promptProviderModel(in *bufio.Reader, out io.Writer, definition providers.ProviderDefinition, options *setupOptions) (string, error) {
	apiKey := ""
	if options.apiKeyEnv != "" {
		apiKey = os.Getenv(options.apiKeyEnv)
	}
	catalog, err := (providers.ModelCatalogClient{}).ListModels(context.Background(), providers.ModelCatalogRequest{
		ProviderID: definition.ID,
		BaseURL:    options.baseURL,
		APIKey:     apiKey,
	})
	if err != nil {
		fmt.Fprintf(out, "  ! Model catalog unavailable: %s\n", err)
		if !definition.SupportsCustomModel {
			return "", err
		}
		return promptString(in, out, "Custom model", "", true)
	}
	if catalog.Message != "" {
		fmt.Fprintf(out, "  ! Using fallback model list: %s\n", catalog.Message)
	}
	fmt.Fprintf(out, "  → %s model catalog: %d model(s) from %s\n", definition.Name, len(catalog.Models), catalog.Source)
	return promptModelFromCatalog(in, out, catalog.Models, definition.SupportsCustomModel)
}

func promptModelFromCatalog(in *bufio.Reader, out io.Writer, models []providers.ModelSummary, allowCustom bool) (string, error) {
	filtered := models
	for {
		printModelChoices(out, filtered)
		answer, err := promptString(in, out, "Select model or search", "", true)
		if err != nil {
			return "", err
		}
		if number, err := strconv.Atoi(answer); err == nil && number >= 1 && number <= minInt(len(filtered), 20) {
			return filtered[number-1].ID, nil
		}
		normalized := strings.TrimSpace(strings.ToLower(answer))
		for _, model := range models {
			if normalized == strings.ToLower(model.ID) || normalized == strings.ToLower(model.Name) {
				return model.ID, nil
			}
		}
		next := filterSetupModels(models, answer)
		if len(next) > 0 {
			filtered = next
			continue
		}
		if allowCustom {
			confirm, err := promptBool(in, out, "Use custom model "+answer+"?", true)
			if err != nil {
				return "", err
			}
			if confirm {
				return answer, nil
			}
		}
		filtered = models
	}
}

func printModelChoices(out io.Writer, models []providers.ModelSummary) {
	limit := minInt(len(models), 20)
	fmt.Fprintf(out, "Models showing %d of %d:\n", limit, len(models))
	for index := 0; index < limit; index++ {
		model := models[index]
		label := model.ID
		if model.Name != "" && model.Name != model.ID {
			label += " - " + model.Name
		}
		if model.Recommended {
			label += " (recommended)"
		}
		if model.ContextWindow > 0 {
			label += fmt.Sprintf(" [%dk context]", model.ContextWindow/1000)
		}
		fmt.Fprintf(out, "  %d. %s\n", index+1, label)
	}
	if len(models) > limit {
		fmt.Fprintln(out, "Type a search term to narrow the full catalog, or paste an exact model id.")
	}
}

func filterSetupModels(models []providers.ModelSummary, query string) []providers.ModelSummary {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return models
	}
	var filtered []providers.ModelSummary
	for _, model := range models {
		value := strings.ToLower(model.ID + " " + model.Name + " " + model.OwnedBy)
		if strings.Contains(value, query) {
			filtered = append(filtered, model)
		}
	}
	return filtered
}

func collectPackOptions(in *bufio.Reader, out io.Writer, options *setupOptions, interactive bool) error {
	if !interactive {
		return validatePack(options.packID)
	}
	fmt.Fprintln(out, "\nStarter pack")
	if interactive {
		choice, err := promptChoice(in, out, "Starter pack", []setupChoice{
			{ID: setupPackDeveloperTeam, Label: "Developer team", Description: "product_pm entrypoint with architecture subagent"},
			{ID: setupPackNone, Label: "None", Description: "Only write provider and sandbox config"},
		}, 0)
		if err != nil {
			return err
		}
		options.packID = choice.ID
	}
	return validatePack(options.packID)
}

func validatePack(packID string) error {
	switch packID {
	case setupPackDeveloperTeam, setupPackNone:
		return nil
	default:
		return fmt.Errorf("unsupported setup pack %q; use developer-team or none", packID)
	}
}

func collectWebToolOptions(in *bufio.Reader, out io.Writer, options *setupOptions, interactive bool) error {
	fmt.Fprintln(out, "\nStep 2/4 - Web Search and Fetch")
	options.webSearch = normalizeToolProvider(options.webSearch)
	options.webFetch = normalizeToolProvider(options.webFetch)
	if interactive {
		search, err := promptChoice(in, out, "Web search provider", []setupChoice{
			{ID: "duckduckgo", Label: "DuckDuckGo", Description: "No API key required"},
			{ID: "brave", Label: "Brave Search", Description: "Requires an API key env var"},
			{ID: "tavily", Label: "Tavily", Description: "Requires an API key env var"},
			{ID: "none", Label: "None", Description: "Do not configure web search"},
		}, 0)
		if err != nil {
			return err
		}
		options.webSearch = search.ID
		if toolProviderNeedsKey(options.webSearch) {
			value, err := promptString(in, out, "Web search API key env var", defaultToolKeyEnv(options.webSearch), true)
			if err != nil {
				return err
			}
			options.webSearchKeyEnv = value
		}

		fetch, err := promptChoice(in, out, "Web fetch provider", []setupChoice{
			{ID: "jina_reader", Label: "Jina Reader", Description: "No API key required"},
			{ID: "direct_http", Label: "Direct HTTP", Description: "No API key required"},
			{ID: "none", Label: "None", Description: "Do not configure web fetch"},
		}, 0)
		if err != nil {
			return err
		}
		options.webFetch = fetch.ID
		if toolProviderNeedsKey(options.webFetch) {
			value, err := promptString(in, out, "Web fetch API key env var", defaultToolKeyEnv(options.webFetch), true)
			if err != nil {
				return err
			}
			options.webFetchKeyEnv = value
		}
	}
	if err := validateWebToolProvider("web search", options.webSearch, []string{"duckduckgo", "brave", "tavily", "none"}); err != nil {
		return err
	}
	if err := validateWebToolProvider("web fetch", options.webFetch, []string{"jina_reader", "direct_http", "none"}); err != nil {
		return err
	}
	if toolProviderNeedsKey(options.webSearch) && strings.TrimSpace(options.webSearchKeyEnv) == "" {
		return fmt.Errorf("--web-search-api-key-env is required for %s", options.webSearch)
	}
	if toolProviderNeedsKey(options.webFetch) && strings.TrimSpace(options.webFetchKeyEnv) == "" {
		return fmt.Errorf("--web-fetch-api-key-env is required for %s", options.webFetch)
	}
	return nil
}

func normalizeToolProvider(provider string) string {
	provider = strings.TrimSpace(strings.ToLower(provider))
	switch provider {
	case "jina-reader", "jina", "jina_reader":
		return "jina_reader"
	case "direct-http", "http", "direct_http":
		return "direct_http"
	default:
		return provider
	}
}

func validateWebToolProvider(label string, provider string, allowed []string) error {
	for _, candidate := range allowed {
		if provider == candidate {
			return nil
		}
	}
	return fmt.Errorf("unsupported %s provider %q", label, provider)
}

func toolProviderNeedsKey(provider string) bool {
	switch normalizeToolProvider(provider) {
	case "brave", "tavily":
		return true
	default:
		return false
	}
}

func defaultToolKeyEnv(provider string) string {
	switch normalizeToolProvider(provider) {
	case "brave":
		return "BRAVE_SEARCH_API_KEY"
	case "tavily":
		return "TAVILY_API_KEY"
	default:
		return ""
	}
}

func collectSandboxOptions(in *bufio.Reader, out io.Writer, options *setupOptions, interactive bool) error {
	fmt.Fprintln(out, "\nStep 3/4 - Execution and sandbox policy")
	if interactive {
		choice, err := promptChoice(in, out, "Sandbox mode", []setupChoice{
			{ID: sandboxModeLocal, Label: "Local workspace", Description: "Per-workspace files with policy approvals"},
			{ID: sandboxModeContainer, Label: "Container sandbox", Description: "Preferred isolation intent when Docker or Podman is available"},
			{ID: sandboxModeNone, Label: "None", Description: "No sandbox policy intent"},
		}, 0)
		if err != nil {
			return err
		}
		options.sandboxMode = choice.ID
	}
	if !knownSandboxMode(options.sandboxMode) {
		return fmt.Errorf("unsupported sandbox mode %q; use local, container, or none", options.sandboxMode)
	}
	if interactive && options.sandboxMode != sandboxModeNone {
		bash, err := promptBool(in, out, "Enable bash command execution?", options.enableBash)
		if err != nil {
			return err
		}
		options.enableBash = bash
		fileWrite, err := promptBool(in, out, "Enable file write tools?", options.enableFileWrite)
		if err != nil {
			return err
		}
		options.enableFileWrite = fileWrite
	}
	return nil
}

func saveSetupProfile(ctx context.Context, options setupOptions) (*providers.Profile, error) {
	providerID := providers.NormalizeProviderID(options.provider)
	definition, ok := providers.GetProviderDefinition(providerID)
	if !ok {
		return nil, fmt.Errorf("unsupported provider %q", options.provider)
	}
	profile := &providers.Profile{
		ID:            slug(options.profileName),
		Name:          options.profileName,
		Kind:          providers.NormalizeKind(definition.AdapterKind),
		BaseURL:       options.baseURL,
		Model:         options.model,
		APIKeyEnv:     options.apiKeyEnv,
		ContextWindow: 0,
		Capabilities: map[string]string{
			"streaming":          "unknown",
			"tool_calling":       "unknown",
			"structured_output":  "unknown",
			"vision":             "unknown",
			"reasoning":          "unknown",
			"provider_id":        providerID,
			"provider_name":      definition.Name,
			"auth_mode":          definition.AuthMode,
			"model_catalog":      definition.CatalogMode,
			"catalog_checked_at": time.Now().UTC().Format(time.RFC3339),
		},
	}
	if profile.Kind == providers.KindOllama {
		profile.APIKeyEnv = ""
		profile.Capabilities["local"] = "true"
	}
	if profile.Kind == providers.KindCodexCLI {
		profile.APIKeyEnv = ""
		profile.Capabilities["local_auth"] = "true"
		profile.Capabilities["execution"] = "codex_cli"
	}
	if profile.Kind == providers.KindClaudeCode {
		profile.APIKeyEnv = ""
		profile.Capabilities["local_auth"] = "true"
		profile.Capabilities["execution"] = "claude_code"
	}
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	db, err := openMigratedDB(options.dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := providers.NewStore(db).Save(ctx, profile); err != nil {
		return nil, fmt.Errorf("save model profile: %w", err)
	}
	return profile, nil
}

func ensureSetupSpec(configPath string) error {
	if _, err := os.Stat(configPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	spec := &agentspec.Spec{
		Version: "0.1",
		Project: agentspec.Project{
			Name:        defaultProjectName(configPath),
			Description: "Created by Nomici setup.",
		},
	}
	return writeSetupSpec(configPath, spec)
}

func writeSandboxConfig(configPath string, sandbox map[string]any) error {
	loaded, exists, err := loadSpecIfExists(configPath)
	if err != nil {
		return err
	}
	if !exists {
		if err := ensureSetupSpec(configPath); err != nil {
			return err
		}
		loaded, _, err = loadSpecIfExists(configPath)
		if err != nil {
			return err
		}
	}
	spec := loaded.Spec
	if spec.Deployment == nil {
		spec.Deployment = map[string]any{}
	}
	spec.Deployment["sandbox"] = sandbox
	return writeSetupSpec(configPath, spec)
}

func writeWebToolConfig(configPath string, tools map[string]map[string]any) error {
	loaded, exists, err := loadSpecIfExists(configPath)
	if err != nil {
		return err
	}
	if !exists {
		if err := ensureSetupSpec(configPath); err != nil {
			return err
		}
		loaded, _, err = loadSpecIfExists(configPath)
		if err != nil {
			return err
		}
	}
	spec := loaded.Spec
	if spec.Tools == nil {
		spec.Tools = map[string]map[string]any{}
	}
	for id, config := range tools {
		if provider, _ := config["provider"].(string); provider == "none" {
			delete(spec.Tools, id)
			continue
		}
		spec.Tools[id] = config
	}
	return writeSetupSpec(configPath, spec)
}

func sandboxConfigFromOptions(options setupOptions) map[string]any {
	return map[string]any{
		"mode":               options.sandboxMode,
		"workspace":          "./workspace",
		"bash_enabled":       options.enableBash && options.sandboxMode != sandboxModeNone,
		"file_write_enabled": options.enableFileWrite && options.sandboxMode != sandboxModeNone,
		"note":               "v0.1 policy intent; runtime adapters enforce capabilities where supported",
	}
}

func webToolConfigFromOptions(options setupOptions) map[string]map[string]any {
	return map[string]map[string]any{
		"web_search": webToolConfig("web_search", options.webSearch, options.webSearchKeyEnv),
		"web_fetch":  webToolConfig("web_fetch", options.webFetch, options.webFetchKeyEnv),
	}
}

func webToolConfig(kind string, provider string, apiKeyEnv string) map[string]any {
	config := map[string]any{
		"kind":      kind,
		"provider":  normalizeToolProvider(provider),
		"mode":      "read_only",
		"status":    "configured",
		"auth":      "none",
		"policy":    "tool-broker-read-only-contract",
		"execution": "configured_not_executed",
		"redaction": "default",
	}
	if strings.TrimSpace(apiKeyEnv) != "" {
		config["auth"] = "api_key_env"
		config["api_key_env"] = strings.TrimSpace(apiKeyEnv)
	}
	return config
}

func compileGraphSnapshot(ctx context.Context, configPath string, dbPath string) (*graph.Snapshot, error) {
	loaded, exists, err := loadSpecIfExists(configPath)
	if err != nil || !exists {
		return nil, err
	}
	if errors := agentspec.Validate(loaded); len(errors) > 0 {
		return nil, fmt.Errorf("AgentSpec validation failed after setup with %d error(s); run `nomici spec validate --config %s`", len(errors), configPath)
	}
	snapshot, errors := graph.Compile(loaded)
	if len(errors) > 0 {
		return nil, fmt.Errorf("AgentGraph validation failed after setup with %d error(s)", len(errors))
	}
	if err := saveGraphSnapshot(ctx, dbPath, snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func printSetupSummary(out io.Writer, options setupOptions, profile *providers.Profile) {
	fmt.Fprintln(out, "\nSetup complete.")
	fmt.Fprintf(out, "  LLM:       %s / %s\n", profile.Kind, profile.Model)
	if profile.APIKeyEnv != "" {
		if _, ok := os.LookupEnv(profile.APIKeyEnv); ok {
			fmt.Fprintf(out, "  Auth:      %s is set\n", profile.APIKeyEnv)
		} else {
			fmt.Fprintf(out, "  Auth:      export %s before model tests or cloud calls\n", profile.APIKeyEnv)
		}
	} else {
		fmt.Fprintln(out, "  Auth:      no API key env required")
	}
	fmt.Fprintf(out, "  Pack:      %s\n", options.packID)
	fmt.Fprintf(out, "  Web search: %s\n", options.webSearch)
	fmt.Fprintf(out, "  Web fetch: %s\n", options.webFetch)
	fmt.Fprintf(out, "  Sandbox:   %s (bash=%t, file_write=%t)\n", options.sandboxMode, options.enableBash && options.sandboxMode != sandboxModeNone, options.enableFileWrite && options.sandboxMode != sandboxModeNone)
	fmt.Fprintf(out, "  Project:   %s (profile references only)\n", options.configPath)
	fmt.Fprintf(out, "  Local DB:  %s\n", options.dbPath)

	fmt.Fprintln(out, "\nNext steps:")
	fmt.Fprintln(out, "  nomici doctor")
	fmt.Fprintf(out, "  nomici dev --config %s\n", options.configPath)
	if options.packID == setupPackDeveloperTeam {
		fmt.Fprintln(out, "  nomici run product_pm \"Plan the next local automation task\"")
	}
}

func promptChoice(in *bufio.Reader, out io.Writer, label string, choices []setupChoice, defaultIndex int) (setupChoice, error) {
	if len(choices) == 0 {
		return setupChoice{}, fmt.Errorf("no choices available for %s", label)
	}
	if defaultIndex < 0 || defaultIndex >= len(choices) {
		defaultIndex = 0
	}
	fmt.Fprintf(out, "%s:\n", label)
	for index, choice := range choices {
		fmt.Fprintf(out, "  %d. %s", index+1, choice.Label)
		if choice.Description != "" {
			fmt.Fprintf(out, " - %s", choice.Description)
		}
		fmt.Fprintln(out)
	}
	answer, err := promptString(in, out, "Enter choice", strconv.Itoa(defaultIndex+1), false)
	if err != nil {
		return setupChoice{}, err
	}
	if number, err := strconv.Atoi(answer); err == nil && number >= 1 && number <= len(choices) {
		return choices[number-1], nil
	}
	normalized := strings.TrimSpace(strings.ToLower(answer))
	for _, choice := range choices {
		if normalized == strings.ToLower(choice.ID) || normalized == strings.ToLower(choice.Label) {
			return choice, nil
		}
	}
	return setupChoice{}, fmt.Errorf("invalid %s choice %q", label, answer)
}

func promptString(in *bufio.Reader, out io.Writer, label string, defaultValue string, required bool) (string, error) {
	if defaultValue != "" {
		fmt.Fprintf(out, "%s [%s]: ", label, defaultValue)
	} else {
		fmt.Fprintf(out, "%s: ", label)
	}
	answer, err := in.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	value := strings.TrimSpace(answer)
	if value == "" {
		value = defaultValue
	}
	if required && value == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return value, nil
}

func promptAPIKeyEnvOrSecret(in *bufio.Reader, out io.Writer, label string, defaultValue string, configPath string) (string, string, error) {
	value, err := promptString(in, out, label, defaultValue, true)
	if err != nil {
		return "", "", err
	}
	if providers.LooksLikeRawSecret(value) {
		envName := strings.TrimSpace(defaultValue)
		if envName == "" {
			envName = "NOMICI_PROVIDER_API_KEY"
		}
		if !providers.ValidEnvVarName(envName) {
			return "", "", fmt.Errorf("default api key env var %q is invalid", envName)
		}
		fmt.Fprintf(out, "  → Raw key will be stored locally in %s as %s\n", localSecretPath(configPath), envName)
		return envName, value, nil
	}
	if err := validateAPIKeyEnv(value); err != nil {
		return "", "", err
	}
	return value, "", nil
}

func validateAPIKeyEnv(value string) error {
	value = strings.TrimSpace(value)
	if providers.LooksLikeRawSecret(value) {
		return fmt.Errorf("api key env var must be an environment variable name, not a raw secret")
	}
	if !providers.ValidEnvVarName(value) {
		return fmt.Errorf("api key env var %q is invalid; use a name like OPENAI_API_KEY", value)
	}
	return nil
}

func localSecretPath(configPath string) string {
	configDir := filepath.Dir(strings.TrimSpace(configPath))
	if configDir == "" || configDir == "." {
		return filepath.Join(".nomici", "secrets.env")
	}
	return filepath.Join(configDir, ".nomici", "secrets.env")
}

func writeLocalSecret(configPath string, envName string, secretValue string) error {
	envName = strings.TrimSpace(envName)
	secretValue = strings.TrimSpace(secretValue)
	if !providers.ValidEnvVarName(envName) {
		return fmt.Errorf("api key env var %q is invalid; use a name like OPENAI_API_KEY", envName)
	}
	if secretValue == "" {
		return fmt.Errorf("local secret value is empty")
	}
	if strings.ContainsAny(secretValue, "\r\n") {
		return fmt.Errorf("local secret value must be a single line")
	}
	path := localSecretPath(configPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create local secret directory: %w", err)
	}
	lines := []string{}
	if data, err := os.ReadFile(path); err == nil {
		if text := strings.TrimRight(string(data), "\r\n"); text != "" {
			lines = strings.Split(text, "\n")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read local secrets: %w", err)
	}
	entry := envName + "=" + secretValue
	replaced := false
	for index, line := range lines {
		key, _, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(key) == envName {
			lines[index] = entry
			replaced = true
		}
	}
	if !replaced {
		lines = append(lines, entry)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		return fmt.Errorf("write local secrets: %w", err)
	}
	return nil
}

func promptBool(in *bufio.Reader, out io.Writer, label string, defaultValue bool) (bool, error) {
	defaultText := "N"
	if defaultValue {
		defaultText = "Y"
	}
	answer, err := promptString(in, out, label+" [Y/N]", defaultText, false)
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes", "true", "1":
		return true, nil
	case "n", "no", "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean answer %q", answer)
	}
}

func knownSandboxMode(mode string) bool {
	switch mode {
	case sandboxModeContainer, sandboxModeLocal, sandboxModeNone:
		return true
	default:
		return false
	}
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
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
	return name
}

func writeSetupSpec(configPath string, spec *agentspec.Spec) error {
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
