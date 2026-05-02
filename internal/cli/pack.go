package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/NomiciAI/nomici-orchestrator/internal/packs"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/spf13/cobra"
)

func newPackCommand() *cobra.Command {
	var dbPath string
	var configPath string
	command := &cobra.Command{
		Use:   "pack",
		Short: "Inspect and install bundled packs",
	}
	command.PersistentFlags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.PersistentFlags().StringVar(&configPath, "config", "nomici.yaml", "AgentSpec config path")
	command.AddCommand(newPackListCommand())
	command.AddCommand(newPackInspectCommand())
	command.AddCommand(newPackInstallCommand(&dbPath, &configPath))
	return command
}

func newPackListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List bundled packs",
		RunE: func(command *cobra.Command, args []string) error {
			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "ID\tNAME\tVERSION\tKIND\tTRUST")
			for _, manifest := range packs.ListBuiltins() {
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", manifest.ID, manifest.Name, manifest.Version, manifest.Kind, manifest.Trust.Level)
			}
			return writer.Flush()
		},
	}
}

func newPackInspectCommand() *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "inspect <pack_id>",
		Short: "Inspect a bundled pack and its permissions",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			manifest, err := packs.RequireBuiltin(args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				encoder := json.NewEncoder(command.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(manifest)
			}
			fmt.Fprintf(command.OutOrStdout(), "ID:          %s\n", manifest.ID)
			fmt.Fprintf(command.OutOrStdout(), "Name:        %s\n", manifest.Name)
			fmt.Fprintf(command.OutOrStdout(), "Version:     %s\n", manifest.Version)
			fmt.Fprintf(command.OutOrStdout(), "Kind:        %s\n", manifest.Kind)
			fmt.Fprintf(command.OutOrStdout(), "Trust:       %s (bundled)\n", manifest.Trust.Level)
			fmt.Fprintf(command.OutOrStdout(), "Entrypoints: %s\n", joinList(manifest.Agents.Entrypoints))
			fmt.Fprintf(command.OutOrStdout(), "Includes:    %s\n", joinList(manifest.Agents.Includes))
			fmt.Fprintf(command.OutOrStdout(), "Optional:    %s\n", joinList(manifest.Agents.Optional))
			fmt.Fprintln(command.OutOrStdout(), "Permissions:")
			fmt.Fprintf(command.OutOrStdout(), "  filesystem.read:  %s\n", joinList(manifest.Permissions.Filesystem.Read))
			fmt.Fprintf(command.OutOrStdout(), "  filesystem.write: %s\n", joinList(manifest.Permissions.Filesystem.Write))
			fmt.Fprintf(command.OutOrStdout(), "  shell:            %s\n", manifest.Permissions.Shell.Mode)
			return nil
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "Print manifest JSON")
	return command
}

func newPackInstallCommand(dbPath *string, configPath *string) *cobra.Command {
	var modelID string
	var force bool
	command := &cobra.Command{
		Use:   "install <pack_id>",
		Short: "Install a bundled pack into nomici.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			switch args[0] {
			case packs.DeveloperTeamID:
				result, err := packs.InstallDeveloperTeam(command.Context(), packs.InstallOptions{
					ConfigPath: *configPath,
					DBPath:     *dbPath,
					ModelID:    modelID,
					Force:      force,
				})
				if err != nil {
					return err
				}
				action := "Updated"
				if result.Created {
					action = "Created"
				}
				fmt.Fprintf(command.OutOrStdout(), "%s %s with pack %s\n", action, result.ConfigPath, result.PackID)
				fmt.Fprintf(command.OutOrStdout(), "Model:      %s\n", result.ModelID)
				fmt.Fprintf(command.OutOrStdout(), "Entrypoint: product_pm\n")
				fmt.Fprintf(command.OutOrStdout(), "Permissions: filesystem read/write ./workspace; shell approval\n")
				snapshot, err := compileGraphFromConfig(result.ConfigPath, command)
				if err != nil {
					return err
				}
				if err := saveGraphSnapshot(command.Context(), *dbPath, snapshot); err != nil {
					return err
				}
				fmt.Fprintf(command.OutOrStdout(), "Graph:      %s\n", snapshot.SnapshotID)
				fmt.Fprintf(command.OutOrStdout(), "Run:        nomici run product_pm \"Plan the next local automation task\"\n")
				fmt.Fprintf(command.OutOrStdout(), "Console:    refresh the Console if this Gateway was started from the same workspace\n")
				return nil
			default:
				_, err := packs.RequireBuiltin(args[0])
				return err
			}
		},
	}
	command.Flags().StringVar(&modelID, "model", "", "Model provider profile ID to use")
	command.Flags().BoolVar(&force, "force", false, "Overwrite pack-managed model or agent definitions")
	return command
}

func joinList(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	result := values[0]
	for _, value := range values[1:] {
		result += ", " + value
	}
	return result
}
