package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/spf13/cobra"
)

func newAgentCommand() *cobra.Command {
	var gatewayURL string
	var configPath string
	var dbPath string
	command := &cobra.Command{
		Use:   "agent",
		Short: "Run and inspect AgentSpec agents",
	}
	command.PersistentFlags().StringVar(&gatewayURL, "gateway-url", defaultGatewayURL, "Nomici Gateway URL")
	command.PersistentFlags().StringVar(&configPath, "config", "nomici.yaml", "AgentSpec config path")
	command.PersistentFlags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.AddCommand(newAgentRunCommand(&configPath, &dbPath, &gatewayURL))
	return command
}

func newAgentRunCommand(configPath *string, dbPath *string, gatewayURL *string) *cobra.Command {
	return &cobra.Command{
		Use:   "run <agent_id> [prompt]",
		Short: "Run an implemented AgentSpec agent",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			prompt := "Say hello from Nomici."
			if len(args) > 1 {
				prompt = strings.Join(args[1:], " ")
			}
			if envURL := os.Getenv("NOMICI_GATEWAY_URL"); envURL != "" && *gatewayURL == defaultGatewayURL {
				*gatewayURL = envURL
			}
			if args[0] == "" {
				return fmt.Errorf("agent_id is required")
			}
			return runGraphEntrypoint(command, *configPath, *dbPath, *gatewayURL, args[0], prompt)
		},
	}
}
