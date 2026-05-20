package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/NomiciAI/nomici-orchestrator/internal/orchestration"
	"github.com/spf13/cobra"
)

func newEvalCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "eval",
		Short: "Run local quality checks for workspace intelligence",
	}
	command.AddCommand(newEvalRouterCommand())
	return command
}

func newEvalRouterCommand() *cobra.Command {
	var prompt string
	command := &cobra.Command{
		Use:   "router",
		Short: "Run chat router eval cases",
		RunE: func(command *cobra.Command, args []string) error {
			cases := routerEvalCases()
			if strings.TrimSpace(prompt) != "" {
				cases = []routerEvalCase{{
					Name:     "custom",
					Prompt:   prompt,
					WantMode: orchestration.ModeWorkspaceRun,
				}}
			}
			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(writer, "CASE\tMODE\tCONFIDENCE\tPASS\tRATIONALE")
			failed := 0
			for _, testCase := range cases {
				decision := orchestration.Route(testCase.Prompt, "", nil)
				pass := routerEvalPass(decision, testCase)
				if !pass {
					failed++
				}
				fmt.Fprintf(writer, "%s\t%s\t%.2f\t%t\t%s\n",
					testCase.Name,
					decision.Mode,
					decision.Confidence,
					pass,
					trimForTable(decision.Rationale, 100),
				)
			}
			if err := writer.Flush(); err != nil {
				return err
			}
			if failed > 0 {
				return fmt.Errorf("router eval failed %d case(s)", failed)
			}
			return nil
		},
	}
	command.Flags().StringVar(&prompt, "prompt", "", "Evaluate a single prompt")
	return command
}

type routerEvalCase struct {
	Name      string
	Prompt    string
	WantMode  string
	WantTools []string
}

func routerEvalCases() []routerEvalCase {
	return []routerEvalCase{
		{Name: "setup-help", Prompt: "how do I start the dev server?", WantMode: orchestration.ModeDirectReply},
		{Name: "clarify-empty", Prompt: "", WantMode: orchestration.ModeClarify},
		{Name: "research", Prompt: "research the current architecture and summarize the tradeoffs", WantMode: orchestration.ModeWorkspaceRun, WantTools: []string{"read_project"}},
		{Name: "code-mutation", Prompt: "implement the fix, edit files, and run tests", WantMode: orchestration.ModeWorkspaceRun, WantTools: []string{"write_project", "run_checks"}},
	}
}

func routerEvalPass(decision orchestration.RouteDecision, testCase routerEvalCase) bool {
	if decision.Mode != testCase.WantMode {
		return false
	}
	for _, tool := range testCase.WantTools {
		if !stringSliceContains(decision.RequiredTools, tool) {
			return false
		}
	}
	return true
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
