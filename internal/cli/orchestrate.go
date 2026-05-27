package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/orchestration"
	"github.com/NomiciAI/nomici-orchestrator/internal/projectconfig"
	"github.com/NomiciAI/nomici-orchestrator/internal/store"
	"github.com/spf13/cobra"
)

func newOrchestrateCommand() *cobra.Command {
	var configPath string
	var dbPath string
	var gatewayURL string
	command := &cobra.Command{
		Use:   "orchestrate",
		Short: "Inspect and edit the default role flow",
	}
	command.PersistentFlags().StringVar(&configPath, "config", "nomici.yaml", "AgentSpec config path")
	command.PersistentFlags().StringVar(&dbPath, "db-path", store.DefaultDBPath, "SQLite database path")
	command.PersistentFlags().StringVar(&gatewayURL, "gateway-url", defaultGatewayURL, "Nomici Gateway URL")
	command.AddCommand(newOrchestrateShowCommand(&configPath))
	command.AddCommand(newOrchestratePreviewCommand(&configPath))
	command.AddCommand(newOrchestrateValidateCommand(&configPath))
	command.AddCommand(newOrchestrateTestCommand(&gatewayURL, &dbPath))
	command.AddCommand(newOrchestrateSetEntrypointCommand(&configPath, &dbPath))
	command.AddCommand(newOrchestrateRoleCommand(&configPath, &dbPath))
	return command
}

func newOrchestrateShowCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show orchestration settings",
		RunE: func(command *cobra.Command, args []string) error {
			config, err := projectconfig.GetOrchestration(*configPath)
			if err != nil {
				return err
			}
			writer := command.OutOrStdout()
			fmt.Fprintf(writer, "Entrypoint: %s\n", emptyDash(config.Entrypoint))
			fmt.Fprintf(writer, "Plan review: %s\n", emptyDash(config.PlanReviewPolicy))
			fmt.Fprintf(writer, "Role order: %s\n", emptyDash(strings.Join(config.RoleOrder, ", ")))
			fmt.Fprintf(writer, "Disabled roles: %s\n", emptyDash(strings.Join(config.DisabledRoles, ", ")))
			return nil
		},
	}
}

func newOrchestratePreviewCommand(configPath *string) *cobra.Command {
	var prompt string
	command := &cobra.Command{
		Use:   "preview",
		Short: "Preview the route decision and sequential role flow",
		RunE: func(command *cobra.Command, args []string) error {
			if strings.TrimSpace(prompt) == "" {
				prompt = "Preview a long-horizon workspace task."
			}
			snapshot, err := compileGraphFromConfig(*configPath, command)
			if err != nil {
				return err
			}
			config, err := projectconfig.GetOrchestration(*configPath)
			if err != nil {
				return err
			}
			entrypoint := config.Entrypoint
			if entrypoint == "" {
				for id := range snapshot.IR.Agents {
					entrypoint = id
					break
				}
			}
			decision := orchestration.Route(prompt, entrypoint, snapshot)
			fmt.Fprintf(command.OutOrStdout(), "Mode:       %s\n", decision.Mode)
			fmt.Fprintf(command.OutOrStdout(), "Complexity: %s\n", decision.Complexity)
			fmt.Fprintf(command.OutOrStdout(), "Entrypoint: %s\n", emptyDash(decision.RecommendedAgentID))
			fmt.Fprintf(command.OutOrStdout(), "Tools:      %s\n", emptyDash(strings.Join(decision.RequiredTools, ", ")))
			if len(config.RoleOrder) > 0 {
				fmt.Fprintf(command.OutOrStdout(), "Role flow:  %s\n", strings.Join(enabledRoles(config), " -> "))
			}
			fmt.Fprintf(command.OutOrStdout(), "Reason:     %s\n", decision.Rationale)
			return nil
		},
	}
	command.Flags().StringVar(&prompt, "prompt", "", "Prompt to preview")
	return command
}

func newOrchestrateValidateCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate that the current orchestration flow has runnable roles",
		RunE: func(command *cobra.Command, args []string) error {
			snapshot, err := compileGraphFromConfig(*configPath, command)
			if err != nil {
				return err
			}
			config, err := projectconfig.GetOrchestration(*configPath)
			if err != nil {
				return err
			}
			roles := enabledRoles(config)
			if len(roles) == 0 {
				for id := range snapshot.IR.Agents {
					roles = append(roles, id)
				}
			}
			for _, role := range roles {
				agent, ok := snapshot.IR.Agents[role]
				if !ok {
					return fmt.Errorf("role %q is not present in AgentGraph", role)
				}
				if agent.Model == "" && agent.Runtime == "" {
					return fmt.Errorf("role %q has no model or runtime", role)
				}
			}
			fmt.Fprintf(command.OutOrStdout(), "Orchestration valid: %d role(s)\n", len(roles))
			return nil
		},
	}
}

func newOrchestrateTestCommand(gatewayURL *string, dbPath *string) *cobra.Command {
	var prompt string
	command := &cobra.Command{
		Use:   "test",
		Short: "Start a real orchestration test session through Gateway",
		RunE: func(command *cobra.Command, args []string) error {
			if strings.TrimSpace(prompt) == "" {
				prompt = "Run a short orchestration test and report the selected roles."
			}
			if envURL := os.Getenv("NOMICI_GATEWAY_URL"); envURL != "" && *gatewayURL == defaultGatewayURL {
				*gatewayURL = envURL
			}
			result, err := postOrchestrationTest(command, *gatewayURL, *dbPath, prompt)
			if err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Status:     %s\n", result.Status)
			if result.GraphSnapshotID != "" {
				fmt.Fprintf(command.OutOrStdout(), "Graph:      %s\n", result.GraphSnapshotID)
			}
			fmt.Fprintf(command.OutOrStdout(), "Entrypoint: %s\n", emptyDash(result.Entrypoint))
			if result.Run != nil {
				fmt.Fprintf(command.OutOrStdout(), "Run ID:     %s\n", result.Run.RunID)
				fmt.Fprintf(command.OutOrStdout(), "Session:    %s\n", result.Run.SessionID)
			}
			fmt.Fprintf(command.OutOrStdout(), "Tasks:      %d\n", len(result.Tasks))
			return nil
		},
	}
	command.Flags().StringVar(&prompt, "prompt", "", "Prompt to run in the test session")
	return command
}

func postOrchestrationTest(command *cobra.Command, gatewayURL string, dbPath string, prompt string) (*orchestrationTestResult, error) {
	baseURL, err := url.Parse(gatewayURL)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(map[string]string{"prompt": prompt})
	if err != nil {
		return nil, err
	}
	endpoint := baseURL.ResolveReference(&url.URL{Path: "/api/orchestration/test"})
	request, err := http.NewRequestWithContext(command.Context(), http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if err := addGatewayAuth(request, dbPath); err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Gateway is not reachable. Remediation: run `nomici dev` or `nomici gateway start`: %w", err)
	}
	defer response.Body.Close()
	var envelope struct {
		Data  orchestrationTestResult `json:"data"`
		Error *struct {
			Code        string `json:"code"`
			Message     string `json:"message"`
			Remediation string `json:"remediation"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode Gateway response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if envelope.Error != nil {
			return nil, fmt.Errorf("%s. Remediation: %s", envelope.Error.Message, envelope.Error.Remediation)
		}
		return nil, fmt.Errorf("Gateway returned HTTP %d", response.StatusCode)
	}
	return &envelope.Data, nil
}

type orchestrationTestResult struct {
	Status          string `json:"status"`
	GraphSnapshotID string `json:"graph_snapshot_id"`
	Entrypoint      string `json:"entrypoint"`
	Run             *struct {
		RunID     string `json:"run_id"`
		SessionID string `json:"session_id"`
	} `json:"run"`
	Tasks []struct {
		AgentID string `json:"agent_id"`
	} `json:"tasks"`
}

func newOrchestrateSetEntrypointCommand(configPath *string, dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "set-entrypoint <agent_id>",
		Short: "Set the default orchestration entrypoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			config, err := projectconfig.GetOrchestration(*configPath)
			if err != nil {
				return err
			}
			config.Entrypoint = args[0]
			if _, err := projectconfig.SaveOrchestration(command.Context(), *configPath, *dbPath, config); err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Entrypoint set: %s\n", args[0])
			return nil
		},
	}
}

func newOrchestrateRoleCommand(configPath *string, dbPath *string) *cobra.Command {
	command := &cobra.Command{
		Use:   "role",
		Short: "Edit role flow membership and ordering",
	}
	command.AddCommand(newOrchestrateRoleEnableCommand(configPath, dbPath, true))
	command.AddCommand(newOrchestrateRoleEnableCommand(configPath, dbPath, false))
	command.AddCommand(newOrchestrateRoleReorderCommand(configPath, dbPath))
	return command
}

func newOrchestrateRoleEnableCommand(configPath *string, dbPath *string, enable bool) *cobra.Command {
	use := "enable <role_id>"
	short := "Enable a role"
	if !enable {
		use = "disable <role_id>"
		short = "Disable a role"
	}
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			config, err := projectconfig.GetOrchestration(*configPath)
			if err != nil {
				return err
			}
			if enable {
				config.DisabledRoles = removeString(config.DisabledRoles, args[0])
			} else if !containsCLI(config.DisabledRoles, args[0]) {
				config.DisabledRoles = append(config.DisabledRoles, args[0])
			}
			if _, err := projectconfig.SaveOrchestration(command.Context(), *configPath, *dbPath, config); err != nil {
				return err
			}
			action := "enabled"
			if !enable {
				action = "disabled"
			}
			fmt.Fprintf(command.OutOrStdout(), "Role %s: %s\n", action, args[0])
			return nil
		},
	}
}

func newOrchestrateRoleReorderCommand(configPath *string, dbPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "reorder <role_id> [role_id...]",
		Short: "Replace the sequential role order",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			config, err := projectconfig.GetOrchestration(*configPath)
			if err != nil {
				return err
			}
			config.RoleOrder = args
			if _, err := projectconfig.SaveOrchestration(command.Context(), *configPath, *dbPath, config); err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Role order: %s\n", strings.Join(args, ", "))
			return nil
		},
	}
}

func containsCLI(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	result := values[:0]
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

func enabledRoles(config projectconfig.OrchestrationConfig) []string {
	disabled := map[string]bool{}
	for _, role := range config.DisabledRoles {
		disabled[role] = true
	}
	roles := []string{}
	for _, role := range config.RoleOrder {
		if role != "" && !disabled[role] {
			roles = append(roles, role)
		}
	}
	return roles
}
