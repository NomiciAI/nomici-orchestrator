package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"
	"unicode"

	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/spf13/cobra"
)

const defaultGatewayURL = "http://127.0.0.1:8787"

func newModelCommand() *cobra.Command {
	var dbPath string
	var gatewayURL string
	command := &cobra.Command{
		Use:   "model",
		Short: "Manage model provider profiles",
	}
	command.PersistentFlags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.PersistentFlags().StringVar(&gatewayURL, "gateway-url", defaultGatewayURL, "Nomici Gateway URL")

	command.AddCommand(newModelSetupCommand(&dbPath))
	command.AddCommand(newModelListCommand(&dbPath))
	command.AddCommand(newModelTestCommand(&gatewayURL))
	command.AddCommand(newModelDoctorCommand(&dbPath))

	return command
}

func newModelDoctorCommand(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check configured model provider profiles",
		RunE: func(command *cobra.Command, args []string) error {
			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			profiles, err := providers.NewStore(db).List(command.Context())
			if err != nil {
				return err
			}
			if len(profiles) == 0 {
				fmt.Fprintln(command.OutOrStdout(), "No model profiles configured.")
				fmt.Fprintln(command.OutOrStdout(), "Remediation: run `nomici model setup --kind openai_compatible --name gpt --model <model> --api-key-env OPENAI_API_KEY`.")
				return nil
			}

			var failed bool
			for _, profile := range profiles {
				status := "ok"
				message := "profile is valid"
				if err := profile.Validate(); err != nil {
					status = "failed"
					message = err.Error()
					failed = true
				} else if profile.APIKeyEnv != "" {
					if _, ok := os.LookupEnv(profile.APIKeyEnv); !ok {
						status = "warning"
						message = "environment variable " + profile.APIKeyEnv + " is not set"
					}
				}
				fmt.Fprintf(command.OutOrStdout(), "%s\t%s\t%s\n", profile.ID, status, message)
			}
			if failed {
				return fmt.Errorf("one or more model profiles failed validation")
			}
			return nil
		},
	}
}

func newModelSetupCommand(dbPath *string) *cobra.Command {
	var kind string
	var provider string
	var name string
	var baseURL string
	var model string
	var apiKeyEnv string
	var contextWindow int

	command := &cobra.Command{
		Use:   "setup",
		Short: "Create or update a model provider profile",
		RunE: func(command *cobra.Command, args []string) error {
			if provider != "" && kind == "" {
				kind = provider
			}
			kind = providers.NormalizeKind(kind)
			if kind == "" {
				kind = providers.KindOpenAICompatible
			}
			if baseURL == "" {
				baseURL = providers.DefaultBaseURL(kind)
			}
			if name == "" {
				name = model
			}
			if name == "" {
				name = kind
			}
			id := slug(name)

			profile := &providers.Profile{
				ID:            id,
				Name:          name,
				Kind:          kind,
				BaseURL:       baseURL,
				Model:         model,
				APIKeyEnv:     apiKeyEnv,
				ContextWindow: contextWindow,
				Capabilities: map[string]string{
					"streaming":         "unknown",
					"tool_calling":      "unknown",
					"structured_output": "unknown",
					"vision":            "unknown",
					"reasoning":         "unknown",
				},
			}
			if kind == providers.KindOllama {
				profile.Capabilities["local"] = "true"
			}

			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			if err := providers.NewStore(db).Save(command.Context(), profile); err != nil {
				return fmt.Errorf("save model profile: %w", err)
			}

			fmt.Fprintf(command.OutOrStdout(), "Saved model profile %q (%s, %s)\n", profile.ID, profile.Kind, profile.Model)
			return nil
		},
	}

	command.Flags().StringVar(&provider, "provider", "", "Provider kind alias (openai-compatible or ollama)")
	command.Flags().StringVar(&kind, "kind", "", "Provider kind (openai_compatible or ollama)")
	command.Flags().StringVar(&name, "name", "", "Provider profile name")
	command.Flags().StringVar(&baseURL, "base-url", "", "Provider base URL")
	command.Flags().StringVar(&model, "model", "", "Model name")
	command.Flags().StringVar(&apiKeyEnv, "api-key-env", "", "Environment variable containing the API key")
	command.Flags().IntVar(&contextWindow, "context-window", 0, "Context window metadata")

	return command
}

func newModelListCommand(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List model provider profiles",
		RunE: func(command *cobra.Command, args []string) error {
			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			profiles, err := providers.NewStore(db).List(command.Context())
			if err != nil {
				return err
			}
			if len(profiles) == 0 {
				fmt.Fprintln(command.OutOrStdout(), "No model profiles configured.")
				return nil
			}

			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "ID\tNAME\tKIND\tMODEL\tAPI KEY SOURCE")
			for _, profile := range profiles {
				apiKeySource := profile.APIKeyEnv
				if apiKeySource == "" {
					apiKeySource = "(none)"
				}
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", profile.ID, profile.Name, profile.Kind, profile.Model, apiKeySource)
			}
			return writer.Flush()
		},
	}
}

func newModelTestCommand(gatewayURL *string) *cobra.Command {
	command := &cobra.Command{
		Use:   "test <profile_id> [prompt]",
		Short: "Test a model provider through Nomici Gateway",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			prompt := "Say hello from Nomici."
			if len(args) > 1 {
				prompt = strings.Join(args[1:], " ")
			}
			if envURL := os.Getenv("NOMICI_GATEWAY_URL"); envURL != "" && *gatewayURL == defaultGatewayURL {
				*gatewayURL = envURL
			}

			result, err := postModelTest(command.Context(), *gatewayURL, args[0], prompt)
			if err != nil {
				return err
			}

			fmt.Fprintf(command.OutOrStdout(), "Run ID:    %s\n", result.RunID)
			fmt.Fprintf(command.OutOrStdout(), "Status:    %s\n", result.Status)
			if len(result.Messages) > 0 {
				fmt.Fprintf(command.OutOrStdout(), "Response:  %s\n", result.Messages[0].Content)
			}
			if result.Usage != nil {
				fmt.Fprintf(command.OutOrStdout(), "Tokens:    %d in / %d out\n", result.Usage.InputTokens, result.Usage.OutputTokens)
			}
			fmt.Fprintf(command.OutOrStdout(), "Trace:     %d events stored\n", result.TraceEventCount)
			return nil
		},
	}
	return command
}

type modelTestEnvelope struct {
	Data struct {
		RunID    string `json:"run_id"`
		Status   string `json:"status"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Usage *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		TraceEventCount int `json:"trace_event_count"`
	} `json:"data"`
	Error *struct {
		Code        string `json:"code"`
		Message     string `json:"message"`
		Remediation string `json:"remediation"`
	} `json:"error"`
}

type modelTestCLIResult struct {
	RunID           string
	Status          string
	Messages        []modelMessage
	Usage           *modelUsage
	TraceEventCount int
}

type modelMessage struct {
	Role    string
	Content string
}

type modelUsage struct {
	InputTokens  int
	OutputTokens int
}

func postModelTest(ctx context.Context, gatewayURL string, profileID string, prompt string) (*modelTestCLIResult, error) {
	baseURL, err := url.Parse(gatewayURL)
	if err != nil {
		return nil, fmt.Errorf("invalid gateway URL: %w", err)
	}
	endpoint := baseURL.ResolveReference(&url.URL{Path: "/api/models/test"})

	body, err := json.Marshal(map[string]any{
		"provider_id": profileID,
		"prompt":      prompt,
		"stream":      false,
	})
	if err != nil {
		return nil, fmt.Errorf("encode model test request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create model test request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Gateway is not reachable. Remediation: run `nomici gateway run`: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read model test response: %w", err)
	}

	var envelope modelTestEnvelope
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return nil, fmt.Errorf("decode model test response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if envelope.Error != nil {
			if envelope.Error.Remediation != "" {
				return nil, fmt.Errorf("%s: %s. Remediation: %s", envelope.Error.Code, envelope.Error.Message, envelope.Error.Remediation)
			}
			return nil, fmt.Errorf("%s: %s", envelope.Error.Code, envelope.Error.Message)
		}
		return nil, fmt.Errorf("model test failed with HTTP %d", response.StatusCode)
	}

	result := &modelTestCLIResult{
		RunID:           envelope.Data.RunID,
		Status:          envelope.Data.Status,
		TraceEventCount: envelope.Data.TraceEventCount,
	}
	for _, message := range envelope.Data.Messages {
		result.Messages = append(result.Messages, modelMessage{Role: message.Role, Content: message.Content})
	}
	if envelope.Data.Usage != nil {
		result.Usage = &modelUsage{InputTokens: envelope.Data.Usage.InputTokens, OutputTokens: envelope.Data.Usage.OutputTokens}
	}
	return result, nil
}

func openMigratedDB(path string) (*sql.DB, error) {
	db, err := store.Open(path)
	if err != nil {
		return nil, err
	}
	if err := store.Migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func slug(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var builder strings.Builder
	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(r)
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if builder.Len() > 0 {
				builder.WriteRune('_')
			}
		}
	}
	result := strings.Trim(builder.String(), "_")
	if result == "" {
		return "model"
	}
	return result
}
