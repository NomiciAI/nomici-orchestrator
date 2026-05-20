package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/spf13/cobra"
)

func newGraphCommand() *cobra.Command {
	var configPath string
	var dbPath string
	command := &cobra.Command{
		Use:   "graph",
		Short: "Compile and inspect AgentGraph snapshots",
	}
	command.PersistentFlags().StringVar(&configPath, "config", "nomici.yaml", "AgentSpec config path")
	command.PersistentFlags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.AddCommand(newGraphValidateCommand(&configPath, &dbPath))
	command.AddCommand(newGraphExportCommand(&configPath))
	return command
}

func newGraphValidateCommand(configPath *string, dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Compile and validate an AgentGraph snapshot",
		RunE: func(command *cobra.Command, args []string) error {
			snapshot, err := compileGraphFromConfig(*configPath, command)
			if err != nil {
				return err
			}
			if err := saveGraphSnapshot(command.Context(), *dbPath, snapshot); err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Graph valid: %s\n", snapshot.SnapshotID)
			return nil
		},
	}
}

func newGraphExportCommand(configPath *string) *cobra.Command {
	var format string
	command := &cobra.Command{
		Use:   "export",
		Short: "Export compiled AgentGraph IR",
		RunE: func(command *cobra.Command, args []string) error {
			if format != "json" {
				return fmt.Errorf("unsupported graph export format %q; only json is implemented", format)
			}
			snapshot, err := compileGraphFromConfig(*configPath, command)
			if err != nil {
				return err
			}
			encoder := json.NewEncoder(command.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(snapshot)
		},
	}
	command.Flags().StringVar(&format, "format", "json", "Export format")
	return command
}

func compileGraphFromConfig(configPath string, command *cobra.Command) (*graph.Snapshot, error) {
	loaded, err := agentspec.LoadFileWithLocal(configPath)
	if err != nil {
		return nil, err
	}
	snapshot, errors := graph.Compile(loaded)
	if len(errors) > 0 {
		printValidationErrors(command, errors)
		return nil, fmt.Errorf("AgentGraph validation failed with %d error(s)", len(errors))
	}
	return snapshot, nil
}

func saveGraphSnapshot(ctx context.Context, dbPath string, snapshot *graph.Snapshot) error {
	db, err := openMigratedDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return graph.NewStore(db).Save(ctx, snapshot)
}
