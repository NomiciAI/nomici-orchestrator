package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/spf13/cobra"
)

func newRunCommand() *cobra.Command {
	var gatewayURL string
	var configPath string
	var dbPath string
	command := &cobra.Command{
		Use:   "run [entrypoint] [prompt]",
		Short: "Run implemented Nomici proof-slice commands",
		Args:  cobra.MinimumNArgs(0),
		RunE: func(command *cobra.Command, args []string) error {
			if len(args) == 0 {
				return command.Help()
			}
			prompt := "Say hello from Nomici."
			if len(args) > 1 {
				prompt = strings.Join(args[1:], " ")
			}
			if envURL := os.Getenv("NOMICI_GATEWAY_URL"); envURL != "" && gatewayURL == defaultGatewayURL {
				gatewayURL = envURL
			}
			return runGraphEntrypoint(command, configPath, dbPath, gatewayURL, args[0], prompt)
		},
	}
	command.PersistentFlags().StringVar(&gatewayURL, "gateway-url", defaultGatewayURL, "Nomici Gateway URL")
	command.PersistentFlags().StringVar(&configPath, "config", "nomici.yaml", "AgentSpec config path")
	command.PersistentFlags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.AddCommand(newRunModelCommand(&gatewayURL))
	return command
}

func newRunModelCommand(gatewayURL *string) *cobra.Command {
	return &cobra.Command{
		Use:   "model <profile_id> [prompt]",
		Short: "Run a model profile through Nomici Gateway",
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
			return nil
		},
	}
}

func runGraphEntrypoint(command *cobra.Command, configPath string, dbPath string, gatewayURL string, entrypoint string, prompt string) error {
	snapshot, err := compileGraphFromConfig(configPath, command)
	if err != nil {
		return err
	}
	agent, ok := snapshot.IR.Agents[entrypoint]
	if !ok {
		return fmt.Errorf("entrypoint %q was not found in AgentGraph", entrypoint)
	}
	if agent.Kind != agentspec.AgentKindGateway && agent.Kind != agentspec.AgentKindModel {
		if agent.Kind != agentspec.AgentKindExternal {
			return fmt.Errorf("agent %q has kind %q, which is not executable in Gate 4; gateway_agent, model_agent, and external_agent backed by cli_agent are supported", entrypoint, agent.Kind)
		}
	}
	outgoing := outgoingEdges(snapshot, entrypoint)

	db, err := openMigratedDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := graph.NewStore(db).Save(command.Context(), snapshot); err != nil {
		return err
	}

	if len(outgoing) > 0 {
		if agent.Kind == agentspec.AgentKindExternal && len(outgoing) == 1 && outgoing[0].Mode == "handoff" {
			return runExternalCLIHandoff(command, snapshot, agent, outgoing[0], configPath, prompt, db)
		}
		return fmt.Errorf("agent %q has outgoing graph edges; Gate 5 only supports one handoff edge between cli_agent-backed external_agent nodes", entrypoint)
	}

	if agent.Kind == agentspec.AgentKindExternal {
		return runExternalCLIAgent(command, snapshot, agent, configPath, prompt, db)
	}

	model, ok := snapshot.IR.Models[agent.Model]
	if !ok {
		return fmt.Errorf("agent %q references missing compiled model %q", entrypoint, agent.Model)
	}
	if err := providers.NewStore(db).Save(command.Context(), graphModelToProvider(model)); err != nil {
		return err
	}

	result, err := postModelTest(command.Context(), gatewayURL, model.ID, graphPrompt(agent, prompt))
	if err != nil {
		return err
	}
	fmt.Fprintf(command.OutOrStdout(), "Graph:     %s\n", snapshot.SnapshotID)
	fmt.Fprintf(command.OutOrStdout(), "Agent:     %s\n", agent.ID)
	fmt.Fprintf(command.OutOrStdout(), "Run ID:    %s\n", result.RunID)
	fmt.Fprintf(command.OutOrStdout(), "Status:    %s\n", result.Status)
	if len(result.Messages) > 0 {
		fmt.Fprintf(command.OutOrStdout(), "Response:  %s\n", result.Messages[0].Content)
	}
	return nil
}
