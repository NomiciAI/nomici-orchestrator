package cli

import (
	"encoding/json"
	"fmt"

	"github.com/NomiciAI/nomici-orchestrator/internal/toolbroker"
	"github.com/spf13/cobra"
)

func newToolCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "tool",
		Short: "Inspect mediated workspace tools",
	}
	command.AddCommand(newToolListCommand())
	command.AddCommand(newToolInspectCommand())
	return command
}

func newToolListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List mediated tools",
		RunE: func(command *cobra.Command, args []string) error {
			writer := command.OutOrStdout()
			fmt.Fprintln(writer, "ID\tAUTH\tFILESYSTEM\tNETWORK\tMUTATION\tEXECUTION")
			for _, definition := range toolbroker.Definitions() {
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\n",
					definition.ID,
					definition.Auth,
					definition.FilesystemRisk,
					definition.NetworkRisk,
					definition.MutationRisk,
					definition.Execution,
				)
			}
			return nil
		},
	}
}

func newToolInspectCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "inspect <tool_id>",
		Aliases: []string{"show"},
		Short:   "Show mediated tool metadata",
		Args:    cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			definition, err := toolbroker.DefinitionByID(args[0])
			if err != nil {
				return err
			}
			payload, err := json.MarshalIndent(definition, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(command.OutOrStdout(), string(payload))
			return nil
		},
	}
}
