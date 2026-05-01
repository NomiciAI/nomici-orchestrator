package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/NomiciAI/nomici-orchestrator/internal/gateway"
	"github.com/NomiciAI/nomici-orchestrator/internal/gatewayauth"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/spf13/cobra"
)

func newGatewayCommand(version string) *cobra.Command {
	command := &cobra.Command{
		Use:   "gateway",
		Short: "Manage Nomici Gateway",
	}

	command.AddCommand(newGatewayRunCommand(version))
	command.AddCommand(newGatewayTokenCommand())

	return command
}

func newGatewayRunCommand(version string) *cobra.Command {
	options := gateway.Options{
		Host:    gateway.DefaultHost,
		Port:    gateway.DefaultPort,
		Version: version,
		DBPath:  store.DefaultDBPath,
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
	command.Flags().StringVar(&options.DBPath, "db-path", options.DBPath, "Gateway SQLite database path")
	command.Flags().StringVar(&options.TokenPath, "token-path", options.TokenPath, "Gateway token file path")

	return command
}

func newGatewayTokenCommand() *cobra.Command {
	var dbPath string
	var tokenPath string
	command := &cobra.Command{
		Use:   "token",
		Short: "Manage the local Gateway token",
	}
	command.PersistentFlags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "Gateway SQLite database path")
	command.PersistentFlags().StringVar(&tokenPath, "token-path", "", "Gateway token file path")
	command.AddCommand(newGatewayTokenShowCommand(&dbPath, &tokenPath))
	command.AddCommand(newGatewayTokenRotateCommand(&dbPath, &tokenPath))
	return command
}

func newGatewayTokenShowCommand(dbPath *string, tokenPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the local Gateway token",
		RunE: func(command *cobra.Command, args []string) error {
			path := resolvedTokenPath(*dbPath, *tokenPath)
			token, err := gatewayauth.LoadForClient(*dbPath)
			if err != nil {
				return fmt.Errorf("read Gateway token: %w. Remediation: run `nomici gateway run` to create it", err)
			}
			if *tokenPath != "" {
				token, err = gatewayauth.Read(path)
				if err != nil {
					return fmt.Errorf("read Gateway token: %w. Remediation: run `nomici gateway run --token-path %s` to create it", err, path)
				}
			}
			fmt.Fprintln(command.OutOrStdout(), token)
			return nil
		},
	}
}

func newGatewayTokenRotateCommand(dbPath *string, tokenPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "rotate",
		Short: "Rotate the local Gateway token file",
		RunE: func(command *cobra.Command, args []string) error {
			path := resolvedTokenPath(*dbPath, *tokenPath)
			if _, err := gatewayauth.Rotate(path); err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Rotated Gateway token at %s\n", path)
			fmt.Fprintln(command.OutOrStdout(), "Restart any running Gateway and refresh Console or CLI sessions that used the old token.")
			return nil
		},
	}
}

func resolvedTokenPath(dbPath string, tokenPath string) string {
	if tokenPath != "" {
		return tokenPath
	}
	return gatewayauth.TokenPathForDB(dbPath)
}
