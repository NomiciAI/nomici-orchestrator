package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/NomiciAI/nomici-orchestrator/internal/sharedcontext"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/spf13/cobra"
)

func newContextCommand() *cobra.Command {
	var dbPath string
	command := &cobra.Command{
		Use:   "context",
		Short: "Inspect Shared Context snapshots",
	}
	command.PersistentFlags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.AddCommand(newContextListCommand(&dbPath))
	return command
}

func newContextListCommand(dbPath *string) *cobra.Command {
	var project string
	var limit int
	command := &cobra.Command{
		Use:   "list",
		Short: "List Shared Context snapshots",
		RunE: func(command *cobra.Command, args []string) error {
			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			snapshots, err := sharedcontext.NewStore(db).ListSnapshots(command.Context(), project, limit)
			if err != nil {
				return err
			}
			if len(snapshots) == 0 {
				fmt.Fprintln(command.OutOrStdout(), "No Shared Context snapshots.")
				return nil
			}
			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "SNAPSHOT ID\tPROJECT\tRUN ID\tFROM\tTO\tSUMMARY")
			for _, snapshot := range snapshots {
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\n",
					snapshot.SnapshotID,
					snapshot.ProjectID,
					snapshot.RunID,
					emptyDash(snapshot.FromAgent),
					emptyDash(snapshot.ToAgent),
					oneLine(snapshot.Summary, 100),
				)
			}
			return writer.Flush()
		},
	}
	command.Flags().StringVar(&project, "project", "", "Filter snapshots by project")
	command.Flags().IntVar(&limit, "limit", 50, "Maximum snapshots to show")
	return command
}

func emptyDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
