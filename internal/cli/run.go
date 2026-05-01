package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newRunCommand() *cobra.Command {
	var gatewayURL string
	command := &cobra.Command{
		Use:   "run",
		Short: "Run implemented Nomici proof-slice commands",
	}
	command.PersistentFlags().StringVar(&gatewayURL, "gateway-url", defaultGatewayURL, "Nomici Gateway URL")
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
