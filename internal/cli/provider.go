package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	"github.com/NomiciAI/nomici-orchestrator/internal/secrets"
	"github.com/spf13/cobra"
)

func newProviderCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "provider",
		Short: "Inspect provider catalog and model availability",
	}
	command.AddCommand(newProviderListCommand())
	command.AddCommand(newProviderModelsCommand())
	command.AddCommand(newProviderDoctorCommand())
	return command
}

func newProviderListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List provider catalog entries",
		RunE: func(command *cobra.Command, args []string) error {
			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "ID\tNAME\tADAPTER\tAUTH\tMODELS\tREADY")
			for _, provider := range providers.ProviderCatalog() {
				ready := "yes"
				if provider.Local && !provider.Available {
					ready = "no"
				}
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\n", provider.ID, provider.Name, provider.AdapterKind, provider.AuthMode, provider.CatalogMode, ready)
			}
			return writer.Flush()
		},
	}
}

func newProviderModelsCommand() *cobra.Command {
	var baseURL string
	var apiKeyEnv string
	var refresh bool
	var search string
	command := &cobra.Command{
		Use:   "models <provider_id>",
		Short: "List provider models from the live catalog when available",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			provider, ok := providers.GetProviderDefinition(args[0])
			if !ok {
				return fmt.Errorf("unknown provider %q", args[0])
			}
			if baseURL == "" {
				baseURL = provider.DefaultBaseURL
			}
			if apiKeyEnv == "" {
				apiKeyEnv = provider.DefaultAPIKeyEnv
			}
			apiKey := ""
			if apiKeyEnv != "" {
				if value, ok := secrets.NewResolver().ResolveEnv(apiKeyEnv); ok {
					apiKey = value
				}
			}
			_ = refresh
			result, err := (providers.ModelCatalogClient{}).ListModels(command.Context(), providers.ModelCatalogRequest{
				ProviderID: provider.ID,
				BaseURL:    baseURL,
				APIKey:     apiKey,
				Query:      search,
			})
			if err != nil {
				return err
			}
			if result.Message != "" {
				fmt.Fprintf(command.ErrOrStderr(), "warning: %s\n", result.Message)
			}
			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "MODEL\tNAME\tSOURCE\tCONTEXT\tRECOMMENDED")
			for _, model := range result.Models {
				contextWindow := ""
				if model.ContextWindow > 0 {
					contextWindow = fmt.Sprintf("%d", model.ContextWindow)
				}
				recommended := ""
				if model.Recommended {
					recommended = "yes"
				}
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", model.ID, model.Name, model.Source, contextWindow, recommended)
			}
			return writer.Flush()
		},
	}
	command.Flags().StringVar(&baseURL, "base-url", "", "Override provider base URL for model discovery")
	command.Flags().StringVar(&apiKeyEnv, "api-key-env", "", "Environment variable containing provider API key")
	command.Flags().BoolVar(&refresh, "refresh", false, "Refresh the provider catalog")
	command.Flags().StringVar(&search, "search", "", "Filter model IDs and names")
	return command
}

func newProviderDoctorCommand() *cobra.Command {
	var baseURL string
	var apiKeyEnv string
	command := &cobra.Command{
		Use:   "doctor <provider_id>",
		Short: "Check provider readiness",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			result := providerDoctor(args[0], baseURL, apiKeyEnv)
			fmt.Fprintf(command.OutOrStdout(), "%s\t%s\t%s\n", result.Name, result.Status, result.Message)
			if result.Status == "failed" {
				return errors.New(result.Message)
			}
			return nil
		},
	}
	command.Flags().StringVar(&baseURL, "base-url", "", "Override provider base URL")
	command.Flags().StringVar(&apiKeyEnv, "api-key-env", "", "Environment variable containing provider API key")
	return command
}

func providerDoctor(providerID string, baseURL string, apiKeyEnv string) doctorResult {
	provider, ok := providers.GetProviderDefinition(providerID)
	if !ok {
		return doctorResult{Name: "provider", Status: "failed", Message: "unknown provider " + providerID}
	}
	if provider.Local {
		if provider.Available {
			return doctorResult{Name: provider.ID, Status: "ok", Message: provider.AvailabilityMessage}
		}
		return doctorResult{Name: provider.ID, Status: "failed", Message: provider.AvailabilityMessage}
	}
	if baseURL == "" {
		baseURL = provider.DefaultBaseURL
	}
	if apiKeyEnv == "" {
		apiKeyEnv = provider.DefaultAPIKeyEnv
	}
	apiKey := ""
	if apiKeyEnv != "" {
		if value, ok := secrets.NewResolver().ResolveEnv(apiKeyEnv); ok {
			apiKey = value
		} else if provider.AuthMode == providers.AuthModeAPIKeyEnv {
			return doctorResult{Name: provider.ID, Status: "warning", Message: apiKeyEnv + " is not set"}
		}
	}
	result, err := (providers.ModelCatalogClient{}).ListModels(context.Background(), providers.ModelCatalogRequest{
		ProviderID: provider.ID,
		BaseURL:    baseURL,
		APIKey:     apiKey,
	})
	if err != nil {
		return doctorResult{Name: provider.ID, Status: "warning", Message: err.Error()}
	}
	message := fmt.Sprintf("%d model(s) available from %s", len(result.Models), result.Source)
	if result.Message != "" {
		message += "; " + result.Message
	}
	return doctorResult{Name: provider.ID, Status: "ok", Message: strings.TrimSpace(message)}
}
