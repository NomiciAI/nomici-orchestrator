package cli

import (
	"fmt"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/skills"
	"github.com/spf13/cobra"
)

func newSkillCommand() *cobra.Command {
	var configPath string
	command := &cobra.Command{
		Use:   "skill",
		Short: "Inspect workspace skills",
	}
	command.PersistentFlags().StringVar(&configPath, "config", "nomici.yaml", "AgentSpec config path")
	command.AddCommand(newSkillListCommand(&configPath))
	command.AddCommand(newSkillInspectCommand(&configPath))
	return command
}

func newSkillListCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List skills",
		RunE: func(command *cobra.Command, args []string) error {
			writer := command.OutOrStdout()
			fmt.Fprintln(writer, "ID\tRISK\tSOURCE\tTOOLS\tDESCRIPTION")
			for _, definition := range skills.List(*configPath) {
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n",
					definition.ID,
					definition.Risk,
					definition.Source,
					emptyDash(strings.Join(definition.RequiredTools, ",")),
					trimForTable(definition.Description, 80),
				)
			}
			return nil
		},
	}
}

func newSkillInspectCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:     "inspect <skill_id>",
		Aliases: []string{"show"},
		Short:   "Show skill metadata",
		Args:    cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			definition, err := skills.Get(*configPath, args[0])
			if err != nil {
				return err
			}
			payload, err := skills.Marshal(definition)
			if err != nil {
				return err
			}
			fmt.Fprintln(command.OutOrStdout(), string(payload))
			return nil
		},
	}
}
