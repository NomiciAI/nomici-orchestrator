package cli

import (
	"fmt"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/spf13/cobra"
)

func newSpecCommand() *cobra.Command {
	var configPath string
	command := &cobra.Command{
		Use:   "spec",
		Short: "Validate AgentSpec files",
	}
	command.PersistentFlags().StringVar(&configPath, "config", "nomici.yaml", "AgentSpec config path")
	command.AddCommand(newSpecValidateCommand(&configPath))
	return command
}

func newSpecValidateCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate an AgentSpec file",
		RunE: func(command *cobra.Command, args []string) error {
			loaded, err := agentspec.LoadFile(*configPath)
			if err != nil {
				return err
			}
			if errors := agentspec.Validate(loaded); len(errors) > 0 {
				printValidationErrors(command, errors)
				return fmt.Errorf("AgentSpec validation failed with %d error(s)", len(errors))
			}
			fmt.Fprintf(command.OutOrStdout(), "AgentSpec valid: %s\n", *configPath)
			return nil
		},
	}
}

func printValidationErrors(command *cobra.Command, errors []agentspec.ValidationError) {
	for _, validationError := range errors {
		source := validationError.Source.File
		if validationError.Source.Line > 0 {
			source = fmt.Sprintf("%s:%d", source, validationError.Source.Line)
		}
		fmt.Fprintf(command.ErrOrStderr(), "%s %s: %s\n", source, validationError.Source.Path, validationError.Message)
		if validationError.Remediation != "" {
			fmt.Fprintf(command.ErrOrStderr(), "Remediation: %s\n", validationError.Remediation)
		}
	}
}
