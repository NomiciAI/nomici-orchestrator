package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	tracepkg "github.com/NomiciAI/nomici-orchestrator/internal/trace"
	"github.com/spf13/cobra"
)

func newTraceCommand() *cobra.Command {
	var dbPath string
	command := &cobra.Command{
		Use:   "trace",
		Short: "Inspect local trace events",
	}
	command.PersistentFlags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.AddCommand(newTraceListCommand(&dbPath))
	command.AddCommand(newTraceShowCommand(&dbPath))
	return command
}

func newTraceListCommand(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List traced runs",
		RunE: func(command *cobra.Command, args []string) error {
			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			runs, err := tracepkg.NewStore(db).ListRuns(command.Context())
			if err != nil {
				return err
			}
			if len(runs) == 0 {
				fmt.Fprintln(command.OutOrStdout(), "No traced runs.")
				return nil
			}

			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "RUN ID\tEVENTS\tLAST TYPE\tLAST TIME")
			for _, run := range runs {
				fmt.Fprintf(writer, "%s\t%d\t%s\t%s\n", run.RunID, run.EventCount, run.LastType, run.LastTime)
			}
			return writer.Flush()
		},
	}
}

func newTraceShowCommand(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "show <run_id>",
		Short: "Show trace events for a run",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			events, err := tracepkg.NewStore(db).ListByRun(command.Context(), args[0])
			if err != nil {
				return err
			}
			if len(events) == 0 {
				return fmt.Errorf("no trace events found for run %q", args[0])
			}
			encoder := json.NewEncoder(command.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(events)
		},
	}
}
