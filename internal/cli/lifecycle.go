package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/gateway"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/lifecycle"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/spf13/cobra"
)

func newUpCommand(version string) *cobra.Command {
	var configPath string
	var dbPath string
	var host string
	var port int
	command := &cobra.Command{
		Use:   "up",
		Short: "Start Nomici Gateway and configured local runtimes",
		RunE: func(command *cobra.Command, args []string) error {
			return startLocalWorkspace(command, version, localStartOptions{
				ConfigPath: configPath,
				DBPath:     dbPath,
				Host:       host,
				Port:       port,
			})
		},
	}
	command.Flags().StringVar(&configPath, "config", "nomici.yaml", "AgentSpec config path")
	command.Flags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.Flags().StringVar(&host, "host", gateway.DefaultHost, "Gateway bind host")
	command.Flags().IntVar(&port, "port", gateway.DefaultPort, "Gateway bind port")
	return command
}

func newDevCommand(version string) *cobra.Command {
	var configPath string
	var dbPath string
	var host string
	var port int
	var noOpen bool
	command := &cobra.Command{
		Use:   "dev",
		Short: "Start the local Nomici workspace and open Console",
		RunE: func(command *cobra.Command, args []string) error {
			if _, err := os.Stat(configPath); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("%s was not found. Remediation: run `nomici setup` first", configPath)
				}
				return err
			}
			if err := startLocalWorkspace(command, version, localStartOptions{
				ConfigPath: configPath,
				DBPath:     dbPath,
				Host:       host,
				Port:       port,
			}); err != nil {
				return err
			}
			consoleURL := fmt.Sprintf("http://%s:%d", host, port)
			if noOpen {
				return nil
			}
			openURL, err := authenticatedConsoleURL(consoleURL, dbPath)
			if err != nil {
				return err
			}
			if err := openBrowser(openURL); err != nil {
				fmt.Fprintf(command.OutOrStdout(), "open\twarning\t%s\n", err)
				fmt.Fprintf(command.OutOrStdout(), "console\t%s\n", consoleURL)
				return nil
			}
			fmt.Fprintf(command.OutOrStdout(), "open\t%s\n", consoleURL)
			return nil
		},
	}
	command.Flags().StringVar(&configPath, "config", "nomici.yaml", "AgentSpec config path")
	command.Flags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.Flags().StringVar(&host, "host", gateway.DefaultHost, "Gateway bind host")
	command.Flags().IntVar(&port, "port", gateway.DefaultPort, "Gateway bind port")
	command.Flags().BoolVar(&noOpen, "no-open", false, "Start without opening Console in the default browser")
	return command
}

type localStartOptions struct {
	ConfigPath string
	DBPath     string
	Host       string
	Port       int
}

func startLocalWorkspace(command *cobra.Command, version string, options localStartOptions) error {
	gatewayState, err := lifecycle.StartGateway(command.Context(), lifecycle.StartGatewayOptions{
		Host:       options.Host,
		Port:       options.Port,
		Version:    version,
		DBPath:     options.DBPath,
		ConfigPath: options.ConfigPath,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(command.OutOrStdout(), "gateway\t%s\tpid=%d\tlog=%s\n", gatewayState.Status, gatewayState.PID, gatewayState.LogPath)
	fmt.Fprintf(command.OutOrStdout(), "console\thttp://%s:%d\topen-command=%q\n", options.Host, options.Port, fmt.Sprintf("nomici gateway open --db-path %s", options.DBPath))

	loaded, exists, err := loadSpecIfExists(options.ConfigPath)
	if err != nil {
		return err
	}
	if !exists {
		fmt.Fprintf(command.OutOrStdout(), "No %s found; Gateway started without project runtimes.\n", options.ConfigPath)
		return nil
	}
	if errors := agentspec.Validate(loaded); len(errors) > 0 {
		printValidationErrors(command, errors)
		return fmt.Errorf("AgentSpec validation failed with %d error(s)", len(errors))
	}
	snapshot, errors := graph.Compile(loaded)
	if len(errors) > 0 {
		printValidationErrors(command, errors)
		return fmt.Errorf("AgentGraph validation failed with %d error(s)", len(errors))
	}
	if err := saveGraphSnapshot(command.Context(), options.DBPath, snapshot); err != nil {
		return err
	}
	fmt.Fprintf(command.OutOrStdout(), "graph\tvalid\tsnapshot=%s\n", snapshot.SnapshotID)

	for id, runtimeSpec := range loaded.Spec.Runtimes {
		switch runtimeSpec.Kind {
		case agentspec.RuntimeKindLocalProcess:
			state, err := lifecycle.StartLocalProcess(command.Context(), lifecycle.StartRuntimeOptions{
				RuntimeID:  id,
				Runtime:    runtimeSpec,
				ConfigPath: options.ConfigPath,
				DBPath:     options.DBPath,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "%s\t%s\tpid=%d\tlog=%s\n", id, state.Status, state.PID, state.LogPath)
		case agentspec.RuntimeKindCLIAgent:
			fmt.Fprintf(command.OutOrStdout(), "%s\tconfigured\tinvoke-only cli_agent\n", id)
		}
	}
	return nil
}

func newDownCommand() *cobra.Command {
	var dbPath string
	command := &cobra.Command{
		Use:   "down",
		Short: "Stop Nomici-managed local runtimes",
		RunE: func(command *cobra.Command, args []string) error {
			paths := lifecycle.NewPaths(dbPath)
			states, err := lifecycle.ListStates(paths)
			if err != nil {
				return err
			}
			if len(states) == 0 {
				fmt.Fprintln(command.OutOrStdout(), "No managed runtimes found.")
				return nil
			}
			for i := len(states) - 1; i >= 0; i-- {
				state := states[i]
				stopped, err := lifecycle.Stop(paths, state.RuntimeID)
				if err != nil {
					return err
				}
				fmt.Fprintf(command.OutOrStdout(), "%s\t%s\n", stopped.RuntimeID, stopped.Status)
			}
			return nil
		},
	}
	command.Flags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	return command
}

func newPSCommand() *cobra.Command {
	var configPath string
	var dbPath string
	command := &cobra.Command{
		Use:   "ps",
		Short: "List Nomici-managed runtime status",
		RunE: func(command *cobra.Command, args []string) error {
			paths := lifecycle.NewPaths(dbPath)
			states, err := lifecycle.ListStates(paths)
			if err != nil {
				return err
			}
			byID := map[string]lifecycle.State{}
			for _, state := range states {
				byID[state.RuntimeID] = state
			}

			loaded, exists, err := loadSpecIfExists(configPath)
			if err != nil {
				return err
			}
			if exists {
				for id, runtimeSpec := range loaded.Spec.Runtimes {
					if _, ok := byID[id]; !ok {
						byID[id] = lifecycle.State{
							RuntimeID:  id,
							Kind:       runtimeSpec.Kind,
							Status:     lifecycle.StatusConfigured,
							Workspace:  runtimeSpec.Workspace,
							ConfigPath: configPath,
						}
					}
				}
			}

			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "NAME\tKIND\tSTATUS\tPID\tLOG")
			for _, state := range sortedStates(byID) {
				pid := "-"
				if state.PID > 0 {
					pid = fmt.Sprintf("%d", state.PID)
				}
				logPath := state.LogPath
				if logPath == "" {
					logPath = "-"
				}
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", state.RuntimeID, state.Kind, state.Status, pid, logPath)
			}
			return writer.Flush()
		},
	}
	command.Flags().StringVar(&configPath, "config", "nomici.yaml", "AgentSpec config path")
	command.Flags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	return command
}

func newLogsCommand() *cobra.Command {
	var dbPath string
	var tail int
	command := &cobra.Command{
		Use:   "logs [runtime_id]",
		Short: "Show logs for a Nomici-managed runtime",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			runtimeID := lifecycle.RuntimeGateway
			if len(args) > 0 {
				runtimeID = args[0]
			}
			state, err := lifecycle.LoadState(lifecycle.NewPaths(dbPath), runtimeID)
			if err != nil {
				return fmt.Errorf("runtime %q has no managed state. Remediation: run `nomici up` first", runtimeID)
			}
			if state.LogPath == "" {
				return fmt.Errorf("runtime %q has no log path", runtimeID)
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
	command.Flags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.Flags().IntVar(&tail, "tail", 200, "Number of log lines to show")
	return command
}

func loadSpecIfExists(configPath string) (*agentspec.LoadedSpec, bool, error) {
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	loaded, err := agentspec.LoadFileWithLocal(configPath)
	if err != nil {
		return nil, true, err
	}
	return loaded, true, nil
}

func sortedStates(byID map[string]lifecycle.State) []lifecycle.State {
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sortStrings(ids)
	states := make([]lifecycle.State, 0, len(ids))
	for _, id := range ids {
		states = append(states, byID[id])
	}
	return states
}

func sortStrings(values []string) {
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if strings.Compare(values[j], values[i]) < 0 {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}

func readTail(path string, limit int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if limit > 0 && len(lines) > limit {
		return lines[len(lines)-limit:], nil
	}
	return lines, nil
}
