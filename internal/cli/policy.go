package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newPolicyCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "policy",
		Short: "Inspect Nomici policy defaults",
	}
	command.AddCommand(newPolicyCheckCommand())
	return command
}

func newPolicyCheckCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Print implemented policy defaults",
		RunE: func(command *cobra.Command, args []string) error {
			fmt.Fprintln(command.OutOrStdout(), "Policy defaults:")
			fmt.Fprintln(command.OutOrStdout(), "- low: read-only cli_agent execution is allowed and traced")
			fmt.Fprintln(command.OutOrStdout(), "- medium: mutable cli_agent workspace execution requires approval")
			fmt.Fprintln(command.OutOrStdout(), "- critical: protected system workspaces are denied")
			fmt.Fprintln(command.OutOrStdout(), "- scopes: once and run")
			return nil
		},
	}
}
