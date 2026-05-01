package cli

import "github.com/spf13/cobra"

func NewRootCommand(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "nomici",
		Short:         "Nomici Orchestrator CLI",
		Long:          "Nomici Orchestrator is a local-first control plane for local and remote AI agents.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.SetVersionTemplate("nomici {{.Version}}\n")
	root.AddCommand(newGatewayCommand(version))
	root.AddCommand(newModelCommand())
	root.AddCommand(newAgentCommand())
	root.AddCommand(newRuntimeCommand())
	root.AddCommand(newContextCommand())
	root.AddCommand(newApprovalsCommand())
	root.AddCommand(newPolicyCommand())
	root.AddCommand(newSpecCommand())
	root.AddCommand(newGraphCommand())
	root.AddCommand(newRunCommand())
	root.AddCommand(newTraceCommand())

	return root
}
