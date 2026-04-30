package cli

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/NomiciAI/nomici-orchestrator/internal/gateway"
	"github.com/spf13/cobra"
)

func newGatewayCommand(version string) *cobra.Command {
	command := &cobra.Command{
		Use:   "gateway",
		Short: "Manage Nomici Gateway",
	}

	command.AddCommand(newGatewayRunCommand(version))

	return command
}

func newGatewayRunCommand(version string) *cobra.Command {
	options := gateway.Options{
		Host:    gateway.DefaultHost,
		Port:    gateway.DefaultPort,
		Version: version,
	}

	command := &cobra.Command{
		Use:   "run",
		Short: "Run Nomici Gateway in the foreground",
		RunE: func(command *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			return gateway.NewServer(options).Run(ctx)
		},
	}

	command.Flags().StringVar(&options.Host, "host", options.Host, "Gateway bind host")
	command.Flags().IntVar(&options.Port, "port", options.Port, "Gateway bind port")

	return command
}
