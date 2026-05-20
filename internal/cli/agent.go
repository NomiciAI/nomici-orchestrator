package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/projectconfig"
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
	command.AddCommand(newAgentListCommand(&configPath))
	command.AddCommand(newAgentShowCommand(&configPath))
	command.AddCommand(newAgentCreateCommand(&configPath, &dbPath))
	command.AddCommand(newAgentUpdateCommand(&configPath, &dbPath))
	command.AddCommand(newAgentDeleteCommand(&configPath, &dbPath))
	command.AddCommand(newAgentRunCommand(&configPath, &dbPath, &gatewayURL))
	return command
}

func newAgentListCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured agents",
		RunE: func(command *cobra.Command, args []string) error {
			agents, err := projectconfig.ListAgents(*configPath)
			if err != nil {
				return err
			}
			writer := command.OutOrStdout()
			fmt.Fprintln(writer, "ID\tKIND\tMODEL/RUNTIME\tROLE")
			for _, agent := range agents {
				target := agent.Model
				if target == "" {
					target = agent.Runtime
				}
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", agent.ID, agent.Kind, emptyDash(target), trimForTable(agent.Role, 64))
			}
			return nil
		},
	}
}

func newAgentShowCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "show <agent_id>",
		Short: "Show an agent definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			agent, err := projectconfig.GetAgent(*configPath, args[0])
			if err != nil {
				return err
			}
			payload, err := json.MarshalIndent(agent, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(command.OutOrStdout(), string(payload))
			return nil
		},
	}
}

func newAgentCreateCommand(configPath *string, dbPath *string) *cobra.Command {
	var record projectconfig.AgentRecord
	command := &cobra.Command{
		Use:   "create <agent_id>",
		Short: "Create a model or external agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			record.ID = args[0]
			if record.Kind == "" {
				record.Kind = agentspec.AgentKindModel
			}
			if _, err := projectconfig.UpsertAgent(command.Context(), *configPath, *dbPath, record); err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Agent saved: %s\n", record.ID)
			return nil
		},
	}
	bindAgentFlags(command, &record)
	return command
}

func newAgentUpdateCommand(configPath *string, dbPath *string) *cobra.Command {
	var update projectconfig.AgentRecord
	command := &cobra.Command{
		Use:   "update <agent_id>",
		Short: "Update an existing agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			existing, err := projectconfig.GetAgent(*configPath, args[0])
			if err != nil {
				return err
			}
			merged := mergeAgentForCLI(*existing, update)
			merged.ID = args[0]
			if _, err := projectconfig.UpsertAgent(command.Context(), *configPath, *dbPath, merged); err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Agent updated: %s\n", merged.ID)
			return nil
		},
	}
	bindAgentFlags(command, &update)
	return command
}

func newAgentDeleteCommand(configPath *string, dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <agent_id>",
		Short: "Delete an agent and its graph edges",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if _, err := projectconfig.DeleteAgent(command.Context(), *configPath, *dbPath, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Agent deleted: %s\n", args[0])
			return nil
		},
	}
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

func bindAgentFlags(command *cobra.Command, record *projectconfig.AgentRecord) {
	command.Flags().StringVar(&record.Name, "name", "", "Display name")
	command.Flags().StringVar(&record.Description, "description", "", "Description")
	command.Flags().StringVar(&record.Kind, "kind", "", "Agent kind: model_agent, gateway_agent, external_agent")
	command.Flags().StringVar(&record.Model, "model", "", "Model reference for model-backed agents")
	command.Flags().StringVar(&record.Runtime, "runtime", "", "Runtime reference for external agents")
	command.Flags().StringVar(&record.Role, "role", "", "Role purpose")
	command.Flags().StringVar(&record.Instructions, "instructions", "", "Agent instructions")
	command.Flags().StringSliceVar(&record.Tools, "tool", nil, "Tool id, repeatable")
	command.Flags().StringSliceVar(&record.Skills, "skill", nil, "Skill id, repeatable")
	command.Flags().StringSliceVar(&record.Tags, "tag", nil, "Tag, repeatable")
	command.Flags().StringSliceVar(&record.Triggers, "trigger", nil, "Trigger phrase, repeatable")
}

func mergeAgentForCLI(existing projectconfig.AgentRecord, update projectconfig.AgentRecord) projectconfig.AgentRecord {
	if update.Name != "" {
		existing.Name = update.Name
	}
	if update.Description != "" {
		existing.Description = update.Description
	}
	if update.Kind != "" {
		existing.Kind = update.Kind
	}
	if update.Model != "" {
		existing.Model = update.Model
	}
	if update.Runtime != "" {
		existing.Runtime = update.Runtime
	}
	if update.Role != "" {
		existing.Role = update.Role
	}
	if update.Instructions != "" {
		existing.Instructions = update.Instructions
	}
	if update.Tools != nil {
		existing.Tools = update.Tools
	}
	if update.Skills != nil {
		existing.Skills = update.Skills
	}
	if update.Tags != nil {
		existing.Tags = update.Tags
	}
	if update.Triggers != nil {
		existing.Triggers = update.Triggers
	}
	return existing
}

func trimForTable(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
}
