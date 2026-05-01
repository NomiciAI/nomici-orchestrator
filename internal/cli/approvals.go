package cli

import (
	"fmt"
	"text/tabwriter"

	policypkg "github.com/NomiciAI/nomici-orchestrator/internal/policy"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	tracepkg "github.com/NomiciAI/nomici-orchestrator/internal/trace"
	"github.com/spf13/cobra"
)

func newApprovalsCommand() *cobra.Command {
	var dbPath string
	command := &cobra.Command{
		Use:   "approvals",
		Short: "Review and resolve pending approvals",
	}
	command.PersistentFlags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.AddCommand(newApprovalsListCommand(&dbPath))
	command.AddCommand(newApprovalsGrantCommand(&dbPath))
	command.AddCommand(newApprovalsDenyCommand(&dbPath))
	return command
}

func newApprovalsListCommand(dbPath *string) *cobra.Command {
	var status string
	command := &cobra.Command{
		Use:   "list",
		Short: "List approval records",
		RunE: func(command *cobra.Command, args []string) error {
			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			approvals, err := policypkg.NewService(db).List(command.Context(), status)
			if err != nil {
				return err
			}
			if len(approvals) == 0 {
				fmt.Fprintln(command.OutOrStdout(), "No approvals.")
				return nil
			}
			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "APPROVAL ID\tSTATUS\tRISK\tSCOPE\tRUN ID\tAGENT\tSUMMARY")
			for _, approval := range approvals {
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					approval.ApprovalID,
					approval.Status,
					approval.Risk,
					emptyDash(approval.Scope),
					approval.RunID,
					emptyDash(approval.RequestedByAgent),
					oneLine(approval.Summary, 100),
				)
			}
			return writer.Flush()
		},
	}
	command.Flags().StringVar(&status, "status", "", "Filter by status: pending, granted, or denied")
	return command
}

func newApprovalsGrantCommand(dbPath *string) *cobra.Command {
	var scope string
	command := &cobra.Command{
		Use:   "grant <approval_id>",
		Short: "Grant a pending approval",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			service := policypkg.NewService(db)
			approval, err := service.Grant(command.Context(), args[0], scope)
			if err != nil {
				return err
			}
			if err := tracepkg.NewStore(db).Append(command.Context(), &tracepkg.Event{
				RunID:     approval.RunID,
				Type:      tracepkg.EventApprovalGranted,
				NodeID:    approval.RequestedByAgent,
				RuntimeID: approval.RuntimeID,
				Payload: jsonPayload(map[string]any{
					"approval_id": approval.ApprovalID,
					"scope":       approval.Scope,
					"risk":        approval.Risk,
					"summary":     approval.Summary,
				}),
			}); err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Granted approval %s with %s scope\n", approval.ApprovalID, approval.Scope)
			return nil
		},
	}
	command.Flags().StringVar(&scope, "scope", policypkg.ScopeOnce, "Approval scope: once or run")
	return command
}

func newApprovalsDenyCommand(dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "deny <approval_id>",
		Short: "Deny a pending approval",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			db, err := openMigratedDB(*dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			service := policypkg.NewService(db)
			approval, err := service.Deny(command.Context(), args[0])
			if err != nil {
				return err
			}
			if err := tracepkg.NewStore(db).Append(command.Context(), &tracepkg.Event{
				RunID:     approval.RunID,
				Type:      tracepkg.EventApprovalDenied,
				NodeID:    approval.RequestedByAgent,
				RuntimeID: approval.RuntimeID,
				Payload: jsonPayload(map[string]any{
					"approval_id": approval.ApprovalID,
					"risk":        approval.Risk,
					"summary":     approval.Summary,
				}),
			}); err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Denied approval %s\n", approval.ApprovalID)
			return nil
		},
	}
}
