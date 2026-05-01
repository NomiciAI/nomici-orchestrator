package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/NomiciAI/nomici-orchestrator/internal/gateway"
	"github.com/NomiciAI/nomici-orchestrator/internal/gatewayauth"
	"github.com/NomiciAI/nomici-orchestrator/internal/lifecycle"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/spf13/cobra"
)

func newGatewayCommand(version string) *cobra.Command {
	command := &cobra.Command{
		Use:   "gateway",
		Short: "Manage Nomici Gateway",
	}

	command.AddCommand(newGatewayRunCommand(version))
	command.AddCommand(newGatewayStartCommand(version))
	command.AddCommand(newGatewayStopCommand())
	command.AddCommand(newGatewayStatusCommand())
	command.AddCommand(newGatewayLogsCommand())
	command.AddCommand(newGatewayOpenCommand())
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

func newGatewayStartCommand(version string) *cobra.Command {
	var dbPath string
	var host string
	var port int
	command := &cobra.Command{
		Use:   "start",
		Short: "Start Nomici Gateway in the background",
		RunE: func(command *cobra.Command, args []string) error {
			state, err := lifecycle.StartGateway(command.Context(), lifecycle.StartGatewayOptions{
				Host:    host,
				Port:    port,
				Version: version,
				DBPath:  dbPath,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Gateway %s pid=%d log=%s\n", state.Status, state.PID, state.LogPath)
			fmt.Fprintf(command.OutOrStdout(), "Console: http://%s:%d\n", host, port)
			return nil
		},
	}
	command.Flags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "Gateway SQLite database path")
	command.Flags().StringVar(&host, "host", gateway.DefaultHost, "Gateway bind host")
	command.Flags().IntVar(&port, "port", gateway.DefaultPort, "Gateway bind port")
	return command
}

func newGatewayStopCommand() *cobra.Command {
	var dbPath string
	command := &cobra.Command{
		Use:   "stop",
		Short: "Stop the background Nomici Gateway",
		RunE: func(command *cobra.Command, args []string) error {
			state, err := lifecycle.Stop(lifecycle.NewPaths(dbPath), lifecycle.RuntimeGateway)
			if err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Gateway %s\n", state.Status)
			return nil
		},
	}
	command.Flags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "Gateway SQLite database path")
	return command
}

func newGatewayStatusCommand() *cobra.Command {
	var gatewayURL string
	command := &cobra.Command{
		Use:   "status",
		Short: "Check Nomici Gateway health",
		RunE: func(command *cobra.Command, args []string) error {
			if envURL := os.Getenv("NOMICI_GATEWAY_URL"); envURL != "" && gatewayURL == defaultGatewayURL {
				gatewayURL = envURL
			}
			health, err := getGatewayHealth(gatewayURL)
			if err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Status:  %s\n", health.Status)
			fmt.Fprintf(command.OutOrStdout(), "Service: %s\n", health.Service)
			fmt.Fprintf(command.OutOrStdout(), "Version: %s\n", health.Version)
			fmt.Fprintf(command.OutOrStdout(), "Time:    %s\n", health.Time)
			return nil
		},
	}
	command.Flags().StringVar(&gatewayURL, "gateway-url", defaultGatewayURL, "Nomici Gateway URL")
	return command
}

func newGatewayLogsCommand() *cobra.Command {
	var dbPath string
	var tail int
	command := &cobra.Command{
		Use:   "logs",
		Short: "Show background Gateway logs",
		RunE: func(command *cobra.Command, args []string) error {
			state, err := lifecycle.LoadState(lifecycle.NewPaths(dbPath), lifecycle.RuntimeGateway)
			if err != nil {
				return fmt.Errorf("Gateway has no managed state. Remediation: run `nomici gateway start` or `nomici up` first")
			}
			lines, err := readTail(state.LogPath, tail)
			if err != nil {
				return err
			}
			for _, line := range lines {
				fmt.Fprintln(command.OutOrStdout(), line)
			}
			return nil
		},
	}
	command.Flags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "Gateway SQLite database path")
	command.Flags().IntVar(&tail, "tail", 200, "Number of log lines to show")
	return command
}

func newGatewayOpenCommand() *cobra.Command {
	var gatewayURL string
	command := &cobra.Command{
		Use:   "open",
		Short: "Open Nomici Console in the default browser",
		RunE: func(command *cobra.Command, args []string) error {
			if envURL := os.Getenv("NOMICI_GATEWAY_URL"); envURL != "" && gatewayURL == defaultGatewayURL {
				gatewayURL = envURL
			}
			if err := openBrowser(gatewayURL); err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Opened %s\n", gatewayURL)
			return nil
		},
	}
	command.Flags().StringVar(&gatewayURL, "gateway-url", defaultGatewayURL, "Nomici Gateway URL")
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

func getGatewayHealth(gatewayURL string) (*gateway.HealthResponse, error) {
	baseURL, err := url.Parse(gatewayURL)
	if err != nil {
		return nil, fmt.Errorf("invalid gateway URL: %w", err)
	}
	endpoint := baseURL.ResolveReference(&url.URL{Path: "/api/health"})
	response, err := http.Get(endpoint.String())
	if err != nil {
		return nil, fmt.Errorf("Gateway is not reachable. Remediation: run `nomici gateway start` or `nomici up`: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gateway health returned HTTP %d", response.StatusCode)
	}
	var health gateway.HealthResponse
	if err := json.NewDecoder(response.Body).Decode(&health); err != nil {
		return nil, fmt.Errorf("decode Gateway health response: %w", err)
	}
	return &health, nil
}

func openBrowser(target string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", target)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		command = exec.Command("xdg-open", target)
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("open browser: %w. Open %s manually", err, target)
	}
	return nil
}
