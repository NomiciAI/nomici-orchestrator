package cli

import (
	"encoding/json"
	"fmt"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/spf13/cobra"
)

func newRuntimeCommand() *cobra.Command {
	var configPath string
	command := &cobra.Command{
		Use:   "runtime",
		Short: "Inspect AgentSpec runtimes",
	}
	command.PersistentFlags().StringVar(&configPath, "config", "nomici.yaml", "AgentSpec config path")
	command.AddCommand(newRuntimeInspectCommand(&configPath))
	return command
}

func newRuntimeInspectCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <runtime_id>",
		Short: "Inspect a runtime from nomici.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			loaded, err := agentspec.LoadFile(*configPath)
			if err != nil {
				return err
			}
			if errors := agentspec.Validate(loaded); len(errors) > 0 {
				printValidationErrors(command, errors)
				return fmt.Errorf("AgentSpec validation failed with %d error(s)", len(errors))
			}
			runtime, ok := loaded.Spec.Runtimes[args[0]]
			if !ok {
				return fmt.Errorf("runtime %q was not found in %s", args[0], *configPath)
			}
			encoder := json.NewEncoder(command.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(runtime)
		},
	}
}
