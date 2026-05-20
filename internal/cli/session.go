package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"text/tabwriter"
	"time"

	runpkg "github.com/NomiciAI/nomici-orchestrator/internal/runs"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/spf13/cobra"
)

func newSessionCommand() *cobra.Command {
	var dbPath string
	command := &cobra.Command{
		Use:   "session",
		Short: "Inspect durable run sessions and task ledgers",
	}
	command.PersistentFlags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.AddCommand(newSessionListCommand(&dbPath))
	command.AddCommand(newSessionShowCommand(&dbPath))
	command.AddCommand(newSessionTasksCommand(&dbPath))
	command.AddCommand(newSessionCancelCommand(&dbPath))
	return command
}

func newSessionListCommand(dbPath *string) *cobra.Command {
	var limit int
	command := &cobra.Command{
		Use:   "list",
		Short: "List durable run sessions",
		RunE: func(command *cobra.Command, args []string) error {
			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			sessions, err := runpkg.NewStore(db).ListSessions(command.Context(), limit)
			if err != nil {
				return err
			}
			if len(sessions) == 0 {
				fmt.Fprintln(command.OutOrStdout(), "No run sessions.")
				return nil
			}
			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "SESSION ID\tRUN ID\tSTATUS\tUPDATED\tTITLE")
			for _, session := range sessions {
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", session.SessionID, session.RunID, session.Status, shortTime(session.UpdatedAt), session.Title)
			}
			return writer.Flush()
		},
	}
	command.Flags().IntVar(&limit, "limit", 20, "Maximum number of sessions to list")
	return command
}

func newSessionShowCommand(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "show <session_id>",
		Short: "Show a durable run session",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			detail, err := runpkg.NewStore(db).GetBySession(command.Context(), args[0])
			if err != nil {
				return sessionLookupError(args[0], err)
			}
			fmt.Fprintf(command.OutOrStdout(), "Session ID: %s\n", detail.Session.SessionID)
			fmt.Fprintf(command.OutOrStdout(), "Run ID:     %s\n", detail.Session.RunID)
			fmt.Fprintf(command.OutOrStdout(), "Status:     %s\n", detail.Session.Status)
			fmt.Fprintf(command.OutOrStdout(), "Project:    %s\n", detail.Session.ProjectID)
			fmt.Fprintf(command.OutOrStdout(), "Graph:      %s\n", detail.Session.GraphSnapshotID)
			fmt.Fprintf(command.OutOrStdout(), "Source:     %s\n", detail.Session.SourceChannel)
			fmt.Fprintf(command.OutOrStdout(), "Started:    %s\n", detail.Session.StartedAt.Format(time.RFC3339))
			fmt.Fprintf(command.OutOrStdout(), "Updated:    %s\n", detail.Session.UpdatedAt.Format(time.RFC3339))
			fmt.Fprintf(command.OutOrStdout(), "Tasks:      %d\n", len(detail.Tasks))
			fmt.Fprintf(command.OutOrStdout(), "Title:      %s\n", detail.Session.Title)
			return nil
		},
	}
}

func newSessionTasksCommand(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "tasks <session_id>",
		Short: "List tasks for a durable run session",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			tasks, err := runpkg.NewStore(db).ListTasksBySession(command.Context(), args[0])
			if err != nil {
				return sessionLookupError(args[0], err)
			}
			if len(tasks) == 0 {
				fmt.Fprintln(command.OutOrStdout(), "No tasks for session.")
				return nil
			}
			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "TASK ID\tAGENT\tRUNTIME\tSTATUS\tUPDATED")
			for _, task := range tasks {
				runtimeID := task.RuntimeID
				if runtimeID == "" {
					runtimeID = "-"
				}
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", task.TaskID, task.AgentID, runtimeID, task.Status, shortTime(task.UpdatedAt))
			}
			return writer.Flush()
		},
	}
}

func newSessionCancelCommand(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <session_id>",
		Short: "Cancel a running durable run session",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			runStore := runpkg.NewStore(db)
			detail, err := runStore.GetBySession(command.Context(), args[0])
			if err != nil {
				return sessionLookupError(args[0], err)
			}
			if err := runStore.CancelSession(command.Context(), args[0]); err != nil {
				return err
			}
			if err := runStore.CancelTasks(command.Context(), detail.Session.RunID); err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Cancelled session %s\n", args[0])
			return nil
		},
	}
}

func sessionLookupError(sessionID string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("session %q was not found", sessionID)
	}
	return err
}

func shortTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format("2006-01-02 15:04:05")
}
