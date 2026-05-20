package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	blockedpkg "github.com/NomiciAI/nomici-orchestrator/internal/blocked"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/spf13/cobra"
)

func newReviewCommand() *cobra.Command {
	var dbPath string
	command := &cobra.Command{
		Use:   "review",
		Short: "Inspect and resolve human review queue items",
	}
	command.PersistentFlags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.AddCommand(newReviewListCommand(&dbPath))
	command.AddCommand(newReviewResolveCommand(&dbPath))
	return command
}

func newReviewListCommand(dbPath *string) *cobra.Command {
	var status string
	var limit int
	command := &cobra.Command{
		Use:   "list",
		Short: "List review queue items",
		RunE: func(command *cobra.Command, args []string) error {
			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			actions, err := blockedpkg.NewStore(db).List(command.Context(), status, limit)
			if err != nil {
				return err
			}
			if len(actions) == 0 {
				fmt.Fprintln(command.OutOrStdout(), "No review queue items.")
				return nil
			}
			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "ID\tSTATUS\tKIND\tSESSION\tTASK\tUPDATED\tTITLE")
			for _, action := range actions {
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					action.BlockedActionID,
					action.Status,
					action.Kind,
					action.SessionID,
					emptyDash(action.TaskID),
					shortTime(action.UpdatedAt),
					trimForTable(action.Title, 80),
				)
			}
			return writer.Flush()
		},
	}
	command.Flags().StringVar(&status, "status", blockedpkg.StatusOpen, "Filter by status")
	command.Flags().IntVar(&limit, "limit", 50, "Maximum number of review items to list")
	return command
}

func newReviewResolveCommand(dbPath *string) *cobra.Command {
	var decision string
	var note string
	command := &cobra.Command{
		Use:   "resolve <blocked_action_id>",
		Short: "Resolve a review queue item",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			decision = strings.ToLower(strings.TrimSpace(decision))
			metadata, _ := json.Marshal(map[string]string{"decision": decision, "note": note})
			store := blockedpkg.NewStore(db)
			var action *blockedpkg.Action
			if decision == "reject" || decision == "stop" {
				action, err = store.Reject(command.Context(), args[0], metadata)
			} else {
				action, err = store.Resolve(command.Context(), args[0], metadata)
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Review item %s: %s\n", action.Status, action.BlockedActionID)
			return nil
		},
	}
	command.Flags().StringVar(&decision, "decision", "resolve", "Decision to record: resolve, retry, skip, reject, or stop")
	command.Flags().StringVar(&note, "note", "", "Reviewer note")
	return command
}
