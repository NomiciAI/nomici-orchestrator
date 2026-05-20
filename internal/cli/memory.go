package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/NomiciAI/nomici-orchestrator/internal/memory"
	"github.com/NomiciAI/nomici-orchestrator/internal/sharedcontext"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/spf13/cobra"
)

func newMemoryCommand() *cobra.Command {
	var dbPath string
	command := &cobra.Command{
		Use:   "memory",
		Short: "Review memory proposals",
	}
	command.PersistentFlags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.AddCommand(newMemoryProposalsCommand(&dbPath))
	command.AddCommand(newMemoryApproveCommand(&dbPath))
	command.AddCommand(newMemoryRejectCommand(&dbPath))
	command.AddCommand(newMemoryDeleteCommand(&dbPath))
	command.AddCommand(newMemoryItemsCommand(&dbPath))
	command.AddCommand(newMemoryItemDeleteCommand(&dbPath))
	return command
}

func newMemoryProposalsCommand(dbPath *string) *cobra.Command {
	var status string
	command := &cobra.Command{
		Use:   "proposals",
		Short: "List memory proposals",
		RunE: func(command *cobra.Command, args []string) error {
			db, err := store.Open(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := store.Migrate(db); err != nil {
				return err
			}
			proposals, err := memory.NewStore(db).List(command.Context(), status, 50)
			if err != nil {
				return err
			}
			if len(proposals) == 0 {
				fmt.Fprintln(command.OutOrStdout(), "No memory proposals.")
				return nil
			}
			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "ID\tSTATUS\tTITLE")
			for _, proposal := range proposals {
				fmt.Fprintf(writer, "%s\t%s\t%s\n", proposal.ProposalID, proposal.Status, trimForTable(proposal.Title, 80))
			}
			return writer.Flush()
		},
	}
	command.Flags().StringVar(&status, "status", "", "Filter by proposal status")
	return command
}

func newMemoryItemsCommand(dbPath *string) *cobra.Command {
	var projectID string
	command := &cobra.Command{
		Use:   "items",
		Short: "List approved reusable memory items",
		RunE: func(command *cobra.Command, args []string) error {
			db, err := store.Open(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := store.Migrate(db); err != nil {
				return err
			}
			if projectID == "" {
				projectID = "default"
			}
			items, err := sharedcontext.NewStore(db).ListItems(command.Context(), projectID, sharedcontext.ScopeProject, 50)
			if err != nil {
				return err
			}
			if len(items) == 0 {
				fmt.Fprintln(command.OutOrStdout(), "No approved memory items.")
				return nil
			}
			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "ID\tTITLE")
			for _, item := range items {
				fmt.Fprintf(writer, "%s\t%s\n", item.ContextID, trimForTable(item.Title, 80))
			}
			return writer.Flush()
		},
	}
	command.Flags().StringVar(&projectID, "project", "", "Project id")
	return command
}

func newMemoryItemDeleteCommand(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "delete-item <context_id>",
		Short: "Delete an approved reusable memory item",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			db, err := store.Open(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := store.Migrate(db); err != nil {
				return err
			}
			if err := sharedcontext.NewStore(db).SetItemStatus(command.Context(), args[0], sharedcontext.StatusDeleted); err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Memory item deleted: %s\n", args[0])
			return nil
		},
	}
}

func newMemoryApproveCommand(dbPath *string) *cobra.Command {
	return memoryTransitionCommand("approve", dbPath)
}

func newMemoryRejectCommand(dbPath *string) *cobra.Command {
	return memoryTransitionCommand("reject", dbPath)
}

func newMemoryDeleteCommand(dbPath *string) *cobra.Command {
	return memoryTransitionCommand("delete", dbPath)
}

func memoryTransitionCommand(action string, dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   action + " <proposal_id>",
		Short: action + " a memory proposal",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			db, err := store.Open(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := store.Migrate(db); err != nil {
				return err
			}
			memoryStore := memory.NewStore(db)
			var proposal *memory.Proposal
			switch action {
			case "approve":
				proposal, err = memoryStore.Approve(command.Context(), args[0], sharedcontext.NewStore(db))
			case "reject":
				proposal, err = memoryStore.Reject(command.Context(), args[0])
			case "delete":
				proposal, err = memoryStore.Delete(command.Context(), args[0])
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Memory proposal %s: %s\n", proposal.Status, proposal.ProposalID)
			return nil
		},
	}
}
