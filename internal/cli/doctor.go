package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/gatewayauth"
	"github.com/NomiciAI/nomici-orchestrator/internal/lifecycle"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	var configPath string
	var dbPath string
	var gatewayURL string
	command := &cobra.Command{
		Use:   "doctor",
		Short: "Check local Nomici configuration and runtime health",
		RunE: func(command *cobra.Command, args []string) error {
			if envURL := os.Getenv("NOMICI_GATEWAY_URL"); envURL != "" && gatewayURL == defaultGatewayURL {
				gatewayURL = envURL
			}
			results := []doctorResult{
				checkToken(dbPath),
				checkGateway(gatewayURL),
				checkAgentSpec(configPath),
				checkProviders(command, dbPath),
				checkManagedRuntimes(dbPath),
			}
			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "CHECK\tSTATUS\tMESSAGE")
			var failed bool
			for _, result := range results {
				fmt.Fprintf(writer, "%s\t%s\t%s\n", result.Name, result.Status, result.Message)
				if result.Status == "failed" {
					failed = true
				}
			}
			if err := writer.Flush(); err != nil {
				return err
			}
			if failed {
				return fmt.Errorf("doctor found failed checks")
			}
			return nil
		},
	}
	command.Flags().StringVar(&configPath, "config", "nomici.yaml", "AgentSpec config path")
	command.Flags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.Flags().StringVar(&gatewayURL, "gateway-url", defaultGatewayURL, "Nomici Gateway URL")
	return command
}

type doctorResult struct {
	Name    string
	Status  string
	Message string
}

func checkToken(dbPath string) doctorResult {
	if _, err := gatewayauth.LoadForClient(dbPath); err != nil {
		return doctorResult{Name: "gateway_token", Status: "warning", Message: "token not found; run `nomici gateway start` or `nomici up`"}
	}
	return doctorResult{Name: "gateway_token", Status: "ok", Message: "token available"}
}

func checkGateway(gatewayURL string) doctorResult {
	health, err := getGatewayHealth(gatewayURL)
	if err != nil {
		return doctorResult{Name: "gateway", Status: "warning", Message: "not reachable; run `nomici up`"}
	}
	return doctorResult{Name: "gateway", Status: "ok", Message: health.Service + " " + health.Version}
}

func checkAgentSpec(configPath string) doctorResult {
	loaded, exists, err := loadSpecIfExists(configPath)
	if err != nil {
		return doctorResult{Name: "agentspec", Status: "failed", Message: err.Error()}
	}
	if !exists {
		return doctorResult{Name: "agentspec", Status: "warning", Message: configPath + " not found"}
	}
	if errors := agentspec.Validate(loaded); len(errors) > 0 {
		return doctorResult{Name: "agentspec", Status: "failed", Message: fmt.Sprintf("%d validation error(s)", len(errors))}
	}
	return doctorResult{Name: "agentspec", Status: "ok", Message: configPath + " valid"}
}

func checkProviders(command *cobra.Command, dbPath string) doctorResult {
	db, err := openMigratedDB(dbPath)
	if err != nil {
		return doctorResult{Name: "models", Status: "failed", Message: err.Error()}
	}
	defer db.Close()
	profiles, err := providers.NewStore(db).List(command.Context())
	if err != nil {
		return doctorResult{Name: "models", Status: "failed", Message: err.Error()}
	}
	if len(profiles) == 0 {
		return doctorResult{Name: "models", Status: "warning", Message: "no model profiles configured"}
	}
	return doctorResult{Name: "models", Status: "ok", Message: fmt.Sprintf("%d profile(s)", len(profiles))}
}

func checkManagedRuntimes(dbPath string) doctorResult {
	states, err := lifecycle.ListStates(lifecycle.NewPaths(dbPath))
	if err != nil {
		return doctorResult{Name: "runtimes", Status: "failed", Message: err.Error()}
	}
	if len(states) == 0 {
		return doctorResult{Name: "runtimes", Status: "warning", Message: "no managed runtime state"}
	}
	var running int
	for _, state := range states {
		if state.Status == lifecycle.StatusRunning {
			running++
		}
	}
	return doctorResult{Name: "runtimes", Status: "ok", Message: fmt.Sprintf("%d managed, %d running", len(states), running)}
}
