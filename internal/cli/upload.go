package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"text/tabwriter"

	runpkg "github.com/NomiciAI/nomici-orchestrator/internal/runs"
	"github.com/NomiciAI/nomici-orchestrator/internal/sandbox"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/NomiciAI/nomici-orchestrator/internal/trace"
	uploadpkg "github.com/NomiciAI/nomici-orchestrator/internal/uploads"
	"github.com/spf13/cobra"
)

func newUploadCommand() *cobra.Command {
	var dbPath string
	command := &cobra.Command{
		Use:   "upload",
		Short: "Manage session uploads",
	}
	command.PersistentFlags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.AddCommand(newUploadAddCommand(&dbPath))
	command.AddCommand(newUploadListCommand(&dbPath))
	return command
}

func newUploadAddCommand(dbPath *string) *cobra.Command {
	var sessionID string
	command := &cobra.Command{
		Use:   "add <path>",
		Short: "Add a file upload to a session workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if sessionID == "" {
				return fmt.Errorf("--session is required")
			}
			sourcePath := args[0]
			info, err := os.Stat(sourcePath)
			if err != nil {
				return err
			}
			if info.IsDir() {
				return fmt.Errorf("upload path must be a file")
			}
			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			runStore := runpkg.NewStore(db)
			detail, err := runStore.GetBySession(command.Context(), sessionID)
			if err != nil {
				return sessionLookupError(sessionID, err)
			}
			sandboxRecord, err := sandbox.NewStore(db).GetByRun(command.Context(), detail.Session.RunID)
			if err != nil {
				return fmt.Errorf("session workspace is not available: %w", err)
			}
			filename := filepath.Base(sourcePath)
			if filename == "." || filename == string(filepath.Separator) || filename == "" {
				return fmt.Errorf("upload path has no filename")
			}
			uploadRoot := filepath.Join(filepath.Dir(sandboxRecord.WorkspaceRoot), "uploads")
			if err := os.MkdirAll(uploadRoot, 0o700); err != nil {
				return err
			}
			target := filepath.Join(uploadRoot, filename)
			if err := copyUploadFile(target, sourcePath); err != nil {
				return err
			}
			contentType := http.DetectContentType(readHeaderBytes(sourcePath))
			upload, err := uploadpkg.NewStore(db).Create(command.Context(), uploadpkg.CreateRequest{
				SessionID:   detail.Session.SessionID,
				RunID:       detail.Session.RunID,
				Filename:    filename,
				Path:        target,
				SizeBytes:   info.Size(),
				ContentType: contentType,
			})
			if err != nil {
				return err
			}
			payload, _ := json.Marshal(map[string]any{
				"upload_id": upload.UploadID,
				"filename":  upload.Filename,
				"size":      upload.SizeBytes,
			})
			if err := trace.NewStore(db).Append(command.Context(), &trace.Event{
				RunID:   detail.Session.RunID,
				Type:    trace.EventUploadCreated,
				Payload: payload,
			}); err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Uploaded %s as %s\n", filename, upload.UploadID)
			return nil
		},
	}
	command.Flags().StringVar(&sessionID, "session", "", "Session id")
	return command
}

func newUploadListCommand(dbPath *string) *cobra.Command {
	var sessionID string
	command := &cobra.Command{
		Use:   "list",
		Short: "List uploads",
		RunE: func(command *cobra.Command, args []string) error {
			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			uploads, err := uploadpkg.NewStore(db).List(command.Context(), sessionID, 50)
			if err != nil {
				return err
			}
			if len(uploads) == 0 {
				fmt.Fprintln(command.OutOrStdout(), "No uploads.")
				return nil
			}
			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "UPLOAD ID\tSESSION\tSIZE\tCREATED\tFILENAME")
			for _, upload := range uploads {
				fmt.Fprintf(writer, "%s\t%s\t%d\t%s\t%s\n", upload.UploadID, upload.SessionID, upload.SizeBytes, shortTime(upload.CreatedAt), upload.Filename)
			}
			return writer.Flush()
		},
	}
	command.Flags().StringVar(&sessionID, "session", "", "Filter by session id")
	return command
}

func copyUploadFile(target string, source string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer output.Close()
	_, err = io.Copy(output, input)
	return err
}

func readHeaderBytes(path string) []byte {
	file, err := os.Open(path)
	if err != nil {
		return []byte{}
	}
	defer file.Close()
	buffer := make([]byte, 512)
	n, _ := file.Read(buffer)
	return buffer[:n]
}
