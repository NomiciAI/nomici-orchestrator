package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"text/tabwriter"

	artifactpkg "github.com/NomiciAI/nomici-orchestrator/internal/artifacts"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/spf13/cobra"
)

func newArtifactCommand() *cobra.Command {
	var dbPath string
	command := &cobra.Command{
		Use:   "artifact",
		Short: "Inspect session artifacts",
	}
	command.PersistentFlags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.AddCommand(newArtifactListCommand(&dbPath))
	command.AddCommand(newArtifactShowCommand(&dbPath))
	return command
}

func newArtifactListCommand(dbPath *string) *cobra.Command {
	var sessionID string
	var limit int
	command := &cobra.Command{
		Use:   "list",
		Short: "List artifacts",
		RunE: func(command *cobra.Command, args []string) error {
			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			artifacts, err := artifactpkg.NewStore(db).List(command.Context(), sessionID, limit)
			if err != nil {
				return err
			}
			if len(artifacts) == 0 {
				fmt.Fprintln(command.OutOrStdout(), "No artifacts.")
				return nil
			}
			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "ARTIFACT ID\tTYPE\tSTATE\tREV\tUPDATED\tTITLE")
			for _, artifact := range artifacts {
				fmt.Fprintf(writer, "%s\t%s\t%s\t%d\t%s\t%s\n", artifact.ArtifactID, artifact.Type, artifact.ReviewState, artifact.Revision, shortTime(artifact.UpdatedAt), artifact.Title)
			}
			return writer.Flush()
		},
	}
	command.Flags().StringVar(&sessionID, "session", "", "Filter by session id")
	command.Flags().IntVar(&limit, "limit", 50, "Maximum number of artifacts to list")
	return command
}

func newArtifactShowCommand(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "show <artifact_id>",
		Short: "Show an artifact",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			artifact, err := artifactpkg.NewStore(db).Get(command.Context(), args[0])
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return fmt.Errorf("artifact %q was not found", args[0])
				}
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Artifact ID: %s\n", artifact.ArtifactID)
			fmt.Fprintf(command.OutOrStdout(), "Session ID:  %s\n", artifact.SessionID)
			fmt.Fprintf(command.OutOrStdout(), "Run ID:      %s\n", artifact.RunID)
			fmt.Fprintf(command.OutOrStdout(), "Task ID:     %s\n", artifact.TaskID)
			fmt.Fprintf(command.OutOrStdout(), "Type:        %s\n", artifact.Type)
			fmt.Fprintf(command.OutOrStdout(), "State:       %s\n", artifact.ReviewState)
			fmt.Fprintf(command.OutOrStdout(), "Revision:    %d\n", artifact.Revision)
			fmt.Fprintf(command.OutOrStdout(), "Path:        %s\n", artifact.Path)
			fmt.Fprintf(command.OutOrStdout(), "Title:       %s\n\n", artifact.Title)
			fmt.Fprintln(command.OutOrStdout(), artifact.Preview)
			return nil
		},
	}
}
