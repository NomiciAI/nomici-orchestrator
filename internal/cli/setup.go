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
	packID          string
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
		sandboxMode:     sandboxModeLocal,
		enableBash:      false,
		enableFileWrite: true,
	}
	command := &cobra.Command{
		Use:   "setup",
		Short: "Configure a usable Nomici workspace",
		Long:  "Configure a usable Nomici workspace by creating a model profile, installing a starter pack, and writing sandbox intent into nomici.yaml.",
		RunE: func(command *cobra.Command, args []string) error {
			return runSetup(command, options)
		},
	}
	command.Flags().StringVar(&options.configPath, "config", options.configPath, "AgentSpec config path")
	command.Flags().StringVar(&options.dbPath, "db-path", options.dbPath, "SQLite database path")
	command.Flags().StringVar(&options.provider, "provider", "", "Provider to configure: openai-compatible or ollama")
	command.Flags().StringVar(&options.profileName, "name", "", "Model profile name")
	command.Flags().StringVar(&options.model, "model", "", "Provider model name")
	command.Flags().StringVar(&options.baseURL, "base-url", "", "Provider base URL")
	command.Flags().StringVar(&options.apiKeyEnv, "api-key-env", "", "Environment variable containing the API key")
	command.Flags().StringVar(&options.packID, "pack", options.packID, "Starter pack to install: developer-team or none")
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
	fmt.Fprintln(out, "This wizard configures the first usable model, starter pack, and sandbox policy for this workspace.")

	if err := collectProviderOptions(in, out, &options, interactive); err != nil {
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
	providerChoices := []setupChoice{
		{ID: providers.KindOpenAICompatible, Label: "OpenAI-compatible endpoint", Description: "OpenAI, OpenRouter, vLLM, LM Studio, SGLang, llama.cpp server"},
		{ID: providers.KindOllama, Label: "Ollama", Description: "Local Ollama server using its OpenAI-compatible endpoint"},
	}
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
	options.provider = providers.NormalizeKind(options.provider)
	if !providers.KnownKind(options.provider) {
		return fmt.Errorf("unsupported provider %q; use openai-compatible or ollama", options.provider)
	}

	if options.baseURL == "" {
		options.baseURL = providers.DefaultBaseURL(options.provider)
	}
	if interactive {
		value, err := promptString(in, out, "Base URL", options.baseURL, false)
		if err != nil {
			return err
		}
		options.baseURL = value
	}

	if strings.TrimSpace(options.model) == "" {
		if !interactive {
			return fmt.Errorf("--model is required when --yes is used")
		}
		value, err := promptString(in, out, "Model", "", true)
		if err != nil {
			return err
		}
		options.model = value
	}

	if options.apiKeyEnv == "" && options.provider == providers.KindOpenAICompatible {
		options.apiKeyEnv = "OPENAI_API_KEY"
	}
	if interactive && options.provider == providers.KindOpenAICompatible {
		value, err := promptString(in, out, "API key env var", options.apiKeyEnv, true)
		if err != nil {
			return err
		}
		options.apiKeyEnv = value
	}
	if options.profileName == "" {
		options.profileName = options.model
	}
	return nil
}

func collectPackOptions(in *bufio.Reader, out io.Writer, options *setupOptions, interactive bool) error {
	fmt.Fprintln(out, "\nStep 2/4 - Choose a starter pack")
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
	switch options.packID {
	case setupPackDeveloperTeam, setupPackNone:
		return nil
	default:
		return fmt.Errorf("unsupported setup pack %q; use developer-team or none", options.packID)
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
	profile := &providers.Profile{
		ID:            slug(options.profileName),
		Name:          options.profileName,
		Kind:          providers.NormalizeKind(options.provider),
		BaseURL:       options.baseURL,
		Model:         options.model,
		APIKeyEnv:     options.apiKeyEnv,
		ContextWindow: 0,
		Capabilities: map[string]string{
			"streaming":         "unknown",
			"tool_calling":      "unknown",
			"structured_output": "unknown",
			"vision":            "unknown",
			"reasoning":         "unknown",
		},
	}
	if profile.Kind == providers.KindOllama {
		profile.APIKeyEnv = ""
		profile.Capabilities["local"] = "true"
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

func sandboxConfigFromOptions(options setupOptions) map[string]any {
	return map[string]any{
		"mode":               options.sandboxMode,
		"workspace":          "./workspace",
		"bash_enabled":       options.enableBash && options.sandboxMode != sandboxModeNone,
		"file_write_enabled": options.enableFileWrite && options.sandboxMode != sandboxModeNone,
		"note":               "v0.1 policy intent; runtime adapters enforce capabilities where supported",
	}
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
	fmt.Fprintf(out, "  Sandbox:   %s (bash=%t, file_write=%t)\n", options.sandboxMode, options.enableBash && options.sandboxMode != sandboxModeNone, options.enableFileWrite && options.sandboxMode != sandboxModeNone)
	fmt.Fprintf(out, "  Config:    %s\n", options.configPath)

	fmt.Fprintln(out, "\nNext steps:")
	fmt.Fprintln(out, "  nomici doctor")
	fmt.Fprintf(out, "  nomici up --config %s\n", options.configPath)
	if options.packID == setupPackDeveloperTeam {
		fmt.Fprintln(out, "  nomici run product_pm \"Plan the next local automation task\"")
	}
	fmt.Fprintln(out, "  nomici gateway open")
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
