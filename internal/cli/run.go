package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/NomiciAI/nomici-orchestrator/internal/adapters"
	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/runs"
	"github.com/NomiciAI/nomici-orchestrator/internal/secrets"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	tracepkg "github.com/NomiciAI/nomici-orchestrator/internal/trace"
	"github.com/spf13/cobra"
)

func newRunCommand() *cobra.Command {
	var gatewayURL string
	var configPath string
	var dbPath string
	command := &cobra.Command{
		Use:   "run [entrypoint] [prompt]",
		Short: "Run implemented Nomici proof-slice commands",
		Args:  cobra.MinimumNArgs(0),
		RunE: func(command *cobra.Command, args []string) error {
			if len(args) == 0 {
				return command.Help()
			}
			prompt := "Say hello from Nomici."
			if len(args) > 1 {
				prompt = strings.Join(args[1:], " ")
			}
			if envURL := os.Getenv("NOMICI_GATEWAY_URL"); envURL != "" && gatewayURL == defaultGatewayURL {
				gatewayURL = envURL
			}
			return runGraphEntrypoint(command, configPath, dbPath, gatewayURL, args[0], prompt)
		},
	}
	command.PersistentFlags().StringVar(&gatewayURL, "gateway-url", defaultGatewayURL, "Nomici Gateway URL")
	command.PersistentFlags().StringVar(&configPath, "config", "nomici.yaml", "AgentSpec config path")
	command.PersistentFlags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.AddCommand(newRunModelCommand(&gatewayURL, &dbPath))
	command.AddCommand(newRunTimelineCommand(&dbPath))
	return command
}

func newRunModelCommand(gatewayURL *string, dbPath *string) *cobra.Command {
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

			result, err := postModelTest(command.Context(), *gatewayURL, *dbPath, args[0], prompt)
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

func newRunTimelineCommand(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "timeline <session_id>",
		Short: "Show a session timeline from tasks and trace events",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			detail, err := runs.NewStore(db).GetBySession(command.Context(), args[0])
			if err != nil {
				return err
			}
			events, err := tracepkg.NewStore(db).ListByRun(command.Context(), detail.Session.RunID)
			if err != nil {
				return err
			}
			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "KIND\tID\tSTATUS/TYPE\tAGENT\tTIME")
			fmt.Fprintf(writer, "session\t%s\t%s\t-\t%s\n", detail.Session.SessionID, detail.Session.Status, shortTime(detail.Session.StartedAt))
			for _, task := range detail.Tasks {
				fmt.Fprintf(writer, "task\t%s\t%s\t%s\t%s\n", task.TaskID, task.Status, task.AgentID, shortTime(task.UpdatedAt))
			}
			for _, event := range events {
				fmt.Fprintf(writer, "trace\t%s\t%s\t%s\t%s\n", event.EventID, event.Type, event.NodeID, shortTime(event.Time))
			}
			return writer.Flush()
		},
	}
}

func runGraphEntrypoint(command *cobra.Command, configPath string, dbPath string, gatewayURL string, entrypoint string, prompt string) error {
	snapshot, err := compileGraphFromConfig(configPath, command)
	if err != nil {
		return err
	}
	agent, ok := snapshot.IR.Agents[entrypoint]
	if !ok {
		return fmt.Errorf("entrypoint %q was not found in AgentGraph", entrypoint)
	}
	if agent.Kind != agentspec.AgentKindGateway && agent.Kind != agentspec.AgentKindModel {
		if agent.Kind != agentspec.AgentKindExternal {
			return fmt.Errorf("agent %q has kind %q, which is not executable in Gate 4; gateway_agent, model_agent, and external_agent backed by cli_agent are supported", entrypoint, agent.Kind)
		}
	}
	outgoing := outgoingEdges(snapshot, entrypoint)

	db, err := openMigratedDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := graph.NewStore(db).Save(command.Context(), snapshot); err != nil {
		return err
	}

	executor := runs.DBExecutor(db, adapters.NewModelAdapter(), secrets.NewResolverForConfig(configPath), configPath)
	result, err := executor.Execute(command.Context(), runs.Request{
		Snapshot: snapshot,
		AgentID:  entrypoint,
		Prompt:   prompt,
	})
	if err != nil {
		return err
	}
	if len(outgoing) > 0 {
		fmt.Fprintf(command.OutOrStdout(), "Graph:     %s\n", snapshot.SnapshotID)
		fmt.Fprintf(command.OutOrStdout(), "Handoff:   %s -> %s\n", agent.ID, outgoing[0].To)
	} else {
		fmt.Fprintf(command.OutOrStdout(), "Graph:     %s\n", snapshot.SnapshotID)
		fmt.Fprintf(command.OutOrStdout(), "Agent:     %s\n", agent.ID)
	}
	if result.RuntimeID != "" {
		fmt.Fprintf(command.OutOrStdout(), "Runtime:   %s\n", result.RuntimeID)
	}
	fmt.Fprintf(command.OutOrStdout(), "Run ID:    %s\n", result.RunID)
	fmt.Fprintf(command.OutOrStdout(), "Status:    %s\n", result.Status)
	if result.ContextSnapshotID != "" {
		fmt.Fprintf(command.OutOrStdout(), "Context:   %s\n", result.ContextSnapshotID)
	}
	if len(result.Messages) > 0 {
		fmt.Fprintf(command.OutOrStdout(), "Response:  %s\n", result.Messages[0].Content)
	}
	if result.CLI != nil {
		if result.CLI.Error != "" {
			fmt.Fprintf(command.OutOrStdout(), "Error:     %s\n", result.CLI.Error)
		}
		if strings.TrimSpace(result.CLI.Stdout) != "" {
			fmt.Fprintf(command.OutOrStdout(), "Response:  %s\n", displayOutput(result.CLI.Stdout, 500))
		}
		if result.CLI.DiffRef != "" {
			fmt.Fprintf(command.OutOrStdout(), "Diff:      %s\n", result.CLI.DiffRef)
		}
		if len(result.CLI.ChangedFiles) > 0 {
			fmt.Fprintf(command.OutOrStdout(), "Changed:   %s\n", strings.Join(result.CLI.ChangedFiles, ", "))
		}
	}
	return nil
}
