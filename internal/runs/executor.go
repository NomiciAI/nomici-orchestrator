package runs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/adapters"
	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/clirunner"
	"github.com/NomiciAI/nomici-orchestrator/internal/graph"
	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
	"github.com/NomiciAI/nomici-orchestrator/internal/policy"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
	"github.com/NomiciAI/nomici-orchestrator/internal/secrets"
	"github.com/NomiciAI/nomici-orchestrator/internal/sharedcontext"
	tracepkg "github.com/NomiciAI/nomici-orchestrator/internal/trace"
)

const previewLimit = 2000

type Executor struct {
	Providers  *providers.Store
	Trace      *tracepkg.Store
	Secrets    *secrets.Resolver
	Adapter    *adapters.OpenAICompatibleAdapter
	Policy     *policy.Service
	Context    *sharedcontext.Store
	ConfigPath string
}

type Request struct {
	Snapshot *graph.Snapshot
	AgentID  string
	Prompt   string
	RunID    string
}

type Result struct {
	RunID             string
	Status            string
	AgentID           string
	RuntimeID         string
	GraphSnapshotID   string
	ContextSnapshotID string
	Messages          []adapters.Message
	CLI               *clirunner.Result
}

func (executor *Executor) Validate(request Request) (*graph.Agent, []graph.Edge, error) {
	if request.Snapshot == nil {
		return nil, nil, fmt.Errorf("no compiled graph snapshot was found")
	}
	if strings.TrimSpace(request.AgentID) == "" {
		return nil, nil, fmt.Errorf("agent_id is required")
	}
	if strings.TrimSpace(request.Prompt) == "" {
		return nil, nil, fmt.Errorf("prompt is required")
	}
	agent, ok := request.Snapshot.IR.Agents[request.AgentID]
	if !ok {
		return nil, nil, fmt.Errorf("agent %q was not found in AgentGraph", request.AgentID)
	}
	if agent.Kind != agentspec.AgentKindGateway && agent.Kind != agentspec.AgentKindModel && agent.Kind != agentspec.AgentKindExternal {
		return nil, nil, fmt.Errorf("agent %q has kind %q, which is not executable", request.AgentID, agent.Kind)
	}
	outgoing := outgoingEdges(request.Snapshot, request.AgentID)
	if len(outgoing) > 0 {
		if agent.Kind == agentspec.AgentKindExternal && len(outgoing) == 1 && outgoing[0].Mode == "handoff" {
			toAgent, ok := request.Snapshot.IR.Agents[outgoing[0].To]
			if !ok {
				return nil, nil, fmt.Errorf("handoff target %q was not found in compiled graph", outgoing[0].To)
			}
			if toAgent.Kind != agentspec.AgentKindExternal {
				return nil, nil, fmt.Errorf("handoff target %q has kind %q; only external_agent targets are supported", toAgent.ID, toAgent.Kind)
			}
			if _, err := cliRuntimeForAgent(request.Snapshot, agent); err != nil {
				return nil, nil, err
			}
			if _, err := cliRuntimeForAgent(request.Snapshot, toAgent); err != nil {
				return nil, nil, err
			}
			return &agent, outgoing, nil
		}
		return nil, nil, fmt.Errorf("agent %q has outgoing graph edges; only one handoff edge between cli_agent-backed external_agent nodes is executable", request.AgentID)
	}
	if agent.Kind == agentspec.AgentKindExternal {
		if _, err := cliRuntimeForAgent(request.Snapshot, agent); err != nil {
			return nil, nil, err
		}
	}
	if agent.Kind == agentspec.AgentKindGateway || agent.Kind == agentspec.AgentKindModel {
		if _, ok := request.Snapshot.IR.Models[agent.Model]; !ok {
			return nil, nil, fmt.Errorf("agent %q references missing compiled model %q", request.AgentID, agent.Model)
		}
	}
	return &agent, outgoing, nil
}

func (executor *Executor) Execute(ctx context.Context, request Request) (*Result, error) {
	agent, outgoing, err := executor.Validate(request)
	if err != nil {
		return nil, err
	}
	if executor.Trace == nil {
		return nil, fmt.Errorf("trace store is not initialized")
	}
	runID := request.RunID
	if runID == "" {
		runID = ids.New("run")
	}
	taskID := ids.New("task")
	result := &Result{
		RunID:           runID,
		Status:          clirunner.StatusFailed,
		AgentID:         agent.ID,
		GraphSnapshotID: request.Snapshot.SnapshotID,
	}

	if len(outgoing) > 0 {
		return executor.executeHandoff(ctx, request, *agent, outgoing[0], runID, taskID)
	}
	if agent.Kind == agentspec.AgentKindExternal {
		return executor.executeExternal(ctx, request, *agent, runID, taskID)
	}
	modelResult, err := executor.executeModel(ctx, request, *agent, runID, taskID)
	if err != nil {
		return result, err
	}
	return modelResult, nil
}

func (executor *Executor) executeModel(ctx context.Context, request Request, agent graph.Agent, runID string, taskID string) (*Result, error) {
	if executor.Secrets == nil || executor.Adapter == nil {
		return nil, fmt.Errorf("model execution services are not initialized")
	}
	model := request.Snapshot.IR.Models[agent.Model]
	profile := graphModelToProvider(model)
	if executor.Providers != nil {
		_ = executor.Providers.Save(ctx, profile)
	}
	if err := executor.Trace.Append(ctx, &tracepkg.Event{
		RunID:  runID,
		Type:   tracepkg.EventRunStarted,
		NodeID: agent.ID,
		Payload: jsonPayload(map[string]any{
			"graph_id": request.Snapshot.SnapshotID,
			"agent_id": agent.ID,
			"task_id":  taskID,
		}),
	}); err != nil {
		return nil, err
	}
	var apiKey string
	var redactions []string
	if profile.APIKeyEnv != "" {
		resolved, ok := executor.Secrets.ResolveEnv(profile.APIKeyEnv)
		if !ok {
			_ = appendModelFailure(ctx, executor.Trace, runID, agent.ID, "missing_secret", "Provider API key environment variable is not set.")
			return nil, fmt.Errorf("provider API key environment variable %s is not set", profile.APIKeyEnv)
		}
		apiKey = resolved
		redactions = append(redactions, secrets.RedactedEnv(profile.APIKeyEnv))
	}
	prompt := GraphPrompt(agent, request.Prompt)
	if err := executor.Trace.Append(ctx, &tracepkg.Event{
		RunID:  runID,
		Type:   tracepkg.EventModelRequested,
		NodeID: agent.ID,
		Payload: jsonPayload(map[string]any{
			"provider_id":    profile.ID,
			"model":          profile.Model,
			"base_url":       profile.BaseURL,
			"prompt":         sharedcontext.RedactText(prompt),
			"api_key_source": profile.APIKeyEnv,
		}),
		Redactions: redactions,
	}); err != nil {
		return nil, err
	}
	invokeResult, err := executor.Adapter.Invoke(ctx, profile.BaseURL, profile.Model, apiKey, adapters.InvokeRequest{
		RunID:    runID,
		NodeID:   agent.ID,
		Messages: []adapters.Message{{Role: "user", Content: prompt}},
		Options:  adapters.InvokeOptions{Stream: false},
	})
	if err != nil {
		_ = appendModelFailure(ctx, executor.Trace, runID, agent.ID, "adapter_failed", err.Error())
		return nil, err
	}
	if invokeResult.Status != adapters.StatusCompleted {
		code := "adapter_failed"
		message := "Provider invocation failed."
		if invokeResult.Error != nil {
			code = invokeResult.Error.Code
			message = invokeResult.Error.Message
		}
		_ = appendModelFailure(ctx, executor.Trace, runID, agent.ID, code, message)
		return &Result{RunID: runID, Status: adapters.StatusFailed, AgentID: agent.ID, GraphSnapshotID: request.Snapshot.SnapshotID, Messages: invokeResult.Messages}, fmt.Errorf("%s: %s", code, message)
	}
	if err := executor.Trace.Append(ctx, &tracepkg.Event{
		RunID:  runID,
		Type:   tracepkg.EventModelCompleted,
		NodeID: agent.ID,
		Payload: jsonPayload(map[string]any{
			"provider_id":    profile.ID,
			"model":          profile.Model,
			"usage":          invokeResult.Usage,
			"messages":       redactMessages(invokeResult.Messages),
			"output_preview": messagePreview(invokeResult.Messages),
		}),
	}); err != nil {
		return nil, err
	}
	if err := executor.Trace.Append(ctx, &tracepkg.Event{
		RunID:  runID,
		Type:   tracepkg.EventRunCompleted,
		NodeID: agent.ID,
	}); err != nil {
		return nil, err
	}
	return &Result{RunID: runID, Status: adapters.StatusCompleted, AgentID: agent.ID, GraphSnapshotID: request.Snapshot.SnapshotID, Messages: invokeResult.Messages}, nil
}

func (executor *Executor) executeExternal(ctx context.Context, request Request, agent graph.Agent, runID string, taskID string) (*Result, error) {
	runtime, err := cliRuntimeForAgent(request.Snapshot, agent)
	if err != nil {
		return nil, err
	}
	if executor.Policy == nil || executor.Context == nil {
		return nil, fmt.Errorf("external execution services are not initialized")
	}
	if err := executor.Trace.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventRunStarted,
		NodeID:    agent.ID,
		RuntimeID: runtime.ID,
		Payload: jsonPayload(map[string]any{
			"graph_id": request.Snapshot.SnapshotID,
			"agent_id": agent.ID,
			"task_id":  taskID,
		}),
	}); err != nil {
		return nil, err
	}
	cliResult, err := executor.invokeExternalCLIAgent(ctx, request.Snapshot.ProjectID, runtime, agent, runID, taskID, GraphPrompt(agent, request.Prompt), nil)
	if err != nil {
		_ = appendExternalFailure(ctx, executor.Trace, runID, agent.ID, runtime.ID, err.Error())
		return nil, err
	}
	contextSnapshot, err := executor.saveCLIContextSnapshot(ctx, request.Snapshot.ProjectID, runID, taskID, agent.ID, "", cliResult, sharedcontext.KindRunSummary)
	if err != nil {
		return nil, err
	}
	runType := tracepkg.EventRunCompleted
	if cliResult.Status != clirunner.StatusCompleted {
		runType = tracepkg.EventRunFailed
	}
	if err := executor.Trace.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      runType,
		NodeID:    agent.ID,
		RuntimeID: runtime.ID,
	}); err != nil {
		return nil, err
	}
	status := cliResult.Status
	if status == "" {
		status = clirunner.StatusFailed
	}
	result := &Result{RunID: runID, Status: status, AgentID: agent.ID, RuntimeID: runtime.ID, GraphSnapshotID: request.Snapshot.SnapshotID, CLI: cliResult}
	if contextSnapshot != nil {
		result.ContextSnapshotID = contextSnapshot.SnapshotID
	}
	if cliResult.Status != clirunner.StatusCompleted {
		return result, fmt.Errorf("cli_agent run failed: %s", cliResult.Error)
	}
	return result, nil
}

func (executor *Executor) executeHandoff(ctx context.Context, request Request, fromAgent graph.Agent, edge graph.Edge, runID string, taskID string) (*Result, error) {
	toAgent := request.Snapshot.IR.Agents[edge.To]
	fromRuntime, err := cliRuntimeForAgent(request.Snapshot, fromAgent)
	if err != nil {
		return nil, err
	}
	toRuntime, err := cliRuntimeForAgent(request.Snapshot, toAgent)
	if err != nil {
		return nil, err
	}
	if executor.Policy == nil || executor.Context == nil {
		return nil, fmt.Errorf("external execution services are not initialized")
	}
	if err := executor.Trace.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventRunStarted,
		NodeID:    fromAgent.ID,
		RuntimeID: fromRuntime.ID,
		Payload: jsonPayload(map[string]any{
			"graph_id": request.Snapshot.SnapshotID,
			"agent_id": fromAgent.ID,
			"task_id":  taskID,
		}),
	}); err != nil {
		return nil, err
	}
	first, err := executor.invokeExternalCLIAgent(ctx, request.Snapshot.ProjectID, fromRuntime, fromAgent, runID, taskID, GraphPrompt(fromAgent, request.Prompt), nil)
	if err != nil {
		_ = appendExternalFailure(ctx, executor.Trace, runID, fromAgent.ID, fromRuntime.ID, err.Error())
		return nil, err
	}
	handoffSnapshot, err := executor.saveCLIContextSnapshot(ctx, request.Snapshot.ProjectID, runID, taskID, fromAgent.ID, toAgent.ID, first, sharedcontext.KindHandoffBriefing)
	if err != nil {
		return nil, err
	}
	if err := appendHandoffCreated(ctx, executor.Trace, runID, fromAgent, toAgent, fromRuntime, edge, handoffSnapshot); err != nil {
		return nil, err
	}
	if err := appendHandoffContextAttached(ctx, executor.Trace, runID, fromAgent, toAgent, toRuntime, handoffSnapshot); err != nil {
		return nil, err
	}
	var second *clirunner.Result
	runType := tracepkg.EventRunCompleted
	if first.Status == clirunner.StatusCompleted {
		second, err = executor.acceptAndInvokeHandoff(ctx, request.Snapshot, fromAgent, toAgent, toRuntime, runID, taskID, request.Prompt, handoffSnapshot)
		if err != nil {
			return nil, err
		}
		if second.Status != clirunner.StatusCompleted {
			runType = tracepkg.EventRunFailed
		}
	} else {
		runType = tracepkg.EventRunFailed
	}
	if err := executor.Trace.Append(ctx, &tracepkg.Event{RunID: runID, Type: runType}); err != nil {
		return nil, err
	}
	status := handoffRunStatus(first, second)
	result := &Result{RunID: runID, Status: status, AgentID: fromAgent.ID, RuntimeID: fromRuntime.ID, GraphSnapshotID: request.Snapshot.SnapshotID, ContextSnapshotID: handoffSnapshot.SnapshotID, CLI: second}
	if first.Status != clirunner.StatusCompleted {
		result.CLI = first
		return result, fmt.Errorf("upstream cli_agent run failed: %s", first.Error)
	}
	if second != nil && second.Status != clirunner.StatusCompleted {
		return result, fmt.Errorf("downstream cli_agent run failed: %s", second.Error)
	}
	return result, nil
}

func (executor *Executor) invokeExternalCLIAgent(ctx context.Context, projectID string, runtime graph.Runtime, agent graph.Agent, runID string, taskID string, prompt string, upstream *sharedcontext.Snapshot) (*clirunner.Result, error) {
	briefing := sharedcontext.RenderBriefing(upstream)
	policyDecision, err := executor.checkCLIAgentPolicy(ctx, projectID, runtime, agent, runID)
	if err != nil {
		return nil, err
	}
	if policyDecision.Decision != policy.DecisionAllow {
		return nil, policyBlockedError(policyDecision)
	}
	if err := executor.Trace.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventAdapterInvoked,
		NodeID:    agent.ID,
		RuntimeID: runtime.ID,
		Payload: jsonPayload(map[string]any{
			"runtime_kind":       runtime.Kind,
			"runner":             runtime.Runner,
			"workspace":          ResolveRuntimeWorkspace(runtime.Workspace, executor.configPath()),
			"executable":         runtime.Invoke.Executable,
			"args_count":         len(runtime.Invoke.Args),
			"env_from":           runtime.EnvFrom,
			"shared_context_ref": briefing.SnapshotID,
		}),
	}); err != nil {
		return nil, err
	}
	result, err := clirunner.Invoke(ctx, CLIRunnerConfig(runtime, agent, executor.configPath()), clirunner.Request{
		RunID:  runID,
		TaskID: taskID,
		Prompt: prompt,
		SharedContext: clirunner.SharedContext{
			SnapshotID: briefing.SnapshotID,
			Briefing:   briefing.Text,
		},
	})
	if err != nil {
		return nil, err
	}
	for _, artifact := range cliArtifacts(result) {
		if err := executor.Trace.Append(ctx, &tracepkg.Event{
			RunID:     runID,
			Type:      tracepkg.EventArtifactCreated,
			NodeID:    agent.ID,
			RuntimeID: runtime.ID,
			Payload:   jsonPayload(artifact),
		}); err != nil {
			return nil, err
		}
	}
	completionPayload := cliCompletionPayload(result)
	eventType := tracepkg.EventAdapterCompleted
	if result.Status != clirunner.StatusCompleted {
		eventType = tracepkg.EventAdapterFailed
	}
	if err := executor.Trace.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      eventType,
		NodeID:    agent.ID,
		RuntimeID: runtime.ID,
		Payload:   jsonPayload(completionPayload),
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func (executor *Executor) checkCLIAgentPolicy(ctx context.Context, projectID string, runtime graph.Runtime, agent graph.Agent, runID string) (*policy.Decision, error) {
	workspace := ResolveRuntimeWorkspace(runtime.Workspace, executor.configPath())
	action := policy.ActionRequest{
		RunID:      runID,
		ActionID:   ids.New("action"),
		ActionType: policy.ActionCLIInvoke,
		ProjectID:  projectID,
		AgentID:    agent.ID,
		RuntimeID:  runtime.ID,
		Workspace:  workspace,
		FilesWrite: RuntimeFilesWrite(runtime),
		Summary:    policySummary(runtime, agent, workspace),
		Subject: map[string]string{
			"workspace": workspace,
			"agent_id":  agent.ID,
			"runtime":   runtime.ID,
		},
	}
	decision, err := executor.Policy.Check(ctx, action)
	if err != nil {
		return nil, err
	}
	if err := appendPolicyTrace(ctx, executor.Trace, runID, agent.ID, runtime.ID, action, decision); err != nil {
		return nil, err
	}
	return decision, nil
}

func (executor *Executor) acceptAndInvokeHandoff(ctx context.Context, snapshot *graph.Snapshot, fromAgent graph.Agent, toAgent graph.Agent, toRuntime graph.Runtime, runID string, taskID string, prompt string, handoffSnapshot *sharedcontext.Snapshot) (*clirunner.Result, error) {
	if err := executor.Trace.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventHandoffAccepted,
		NodeID:    toAgent.ID,
		RuntimeID: toRuntime.ID,
		Payload: jsonPayload(map[string]any{
			"from":        fromAgent.ID,
			"to":          toAgent.ID,
			"snapshot_id": handoffSnapshot.SnapshotID,
		}),
	}); err != nil {
		return nil, err
	}
	result, err := executor.invokeExternalCLIAgent(ctx, snapshot.ProjectID, toRuntime, toAgent, runID, taskID, GraphPrompt(toAgent, prompt), handoffSnapshot)
	if err != nil {
		_ = appendExternalFailure(ctx, executor.Trace, runID, toAgent.ID, toRuntime.ID, err.Error())
		return nil, err
	}
	if _, err := executor.saveCLIContextSnapshot(ctx, snapshot.ProjectID, runID, taskID, toAgent.ID, "", result, sharedcontext.KindRunSummary); err != nil {
		return nil, err
	}
	return result, nil
}

func (executor *Executor) saveCLIContextSnapshot(ctx context.Context, projectID string, runID string, taskID string, fromAgent string, toAgent string, result *clirunner.Result, kind string) (*sharedcontext.Snapshot, error) {
	snapshot := &sharedcontext.Snapshot{
		ProjectID:       projectID,
		RunID:           runID,
		TaskID:          taskID,
		FromAgent:       fromAgent,
		ToAgent:         toAgent,
		Summary:         cliContextSummary(result),
		Decisions:       cliContextDecisions(result),
		OpenIssues:      cliOpenIssues(result),
		Recommendations: cliRecommendations(toAgent, result),
		ArtifactRefs:    cliArtifactRefs(result),
		CreatedBy:       sharedcontext.CreatedBy{Kind: "gateway_generated", AgentID: fromAgent},
	}
	if kind == sharedcontext.KindHandoffBriefing && toAgent != "" {
		snapshot.ContextItemRefs = []string{}
	}
	if err := executor.Context.SaveSnapshot(ctx, snapshot); err != nil {
		return nil, err
	}
	if err := executor.Trace.Append(ctx, &tracepkg.Event{
		RunID:   runID,
		Type:    tracepkg.EventContextSnapshotCreated,
		NodeID:  fromAgent,
		Payload: jsonPayload(contextSnapshotPayload(snapshot, kind)),
	}); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (executor *Executor) configPath() string {
	if strings.TrimSpace(executor.ConfigPath) == "" {
		return "nomici.yaml"
	}
	return executor.ConfigPath
}

func GraphPrompt(agent graph.Agent, prompt string) string {
	parts := []string{}
	if agent.Role != "" {
		parts = append(parts, "Role:\n"+agent.Role)
	}
	if agent.Instructions != "" {
		parts = append(parts, "Instructions:\n"+agent.Instructions)
	}
	parts = append(parts, "Task:\n"+prompt)
	return strings.Join(parts, "\n\n")
}

func CLIRunnerConfig(runtime graph.Runtime, agent graph.Agent, configPath string) clirunner.Config {
	return clirunner.Config{
		RuntimeID:      runtime.ID,
		AgentID:        agent.ID,
		Workspace:      ResolveRuntimeWorkspace(runtime.Workspace, configPath),
		Executable:     runtime.Invoke.Executable,
		Args:           runtime.Invoke.Args,
		Stdin:          runtime.Invoke.Stdin,
		Env:            runtime.Env,
		EnvFrom:        runtime.EnvFrom,
		TimeoutSeconds: runtime.TimeoutSeconds,
		FilesWrite:     RuntimeFilesWrite(runtime),
	}
}

func ResolveRuntimeWorkspace(workspace string, configPath string) string {
	if strings.TrimSpace(workspace) == "" {
		workspace = "."
	}
	if filepath.IsAbs(workspace) {
		return workspace
	}
	configDir := filepath.Dir(configPath)
	if configDir == "." || configDir == "" {
		return workspace
	}
	return filepath.Join(configDir, workspace)
}

func RuntimeFilesWrite(runtime graph.Runtime) bool {
	value, ok := runtime.Capabilities["files_write"]
	if !ok {
		return true
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true") || strings.EqualFold(typed, "yes")
	default:
		return true
	}
}

func graphModelToProvider(model graph.Model) *providers.Profile {
	capabilities := map[string]string{}
	for _, capability := range model.Capabilities {
		capabilities[capability] = "true"
	}
	return &providers.Profile{
		ID:            model.ID,
		Name:          model.ID,
		Kind:          model.Kind,
		BaseURL:       model.BaseURL,
		Model:         model.Model,
		APIKeyEnv:     model.APIKeyEnv,
		Capabilities:  capabilities,
		ContextWindow: model.ContextWindow,
	}
}

func cliRuntimeForAgent(snapshot *graph.Snapshot, agent graph.Agent) (graph.Runtime, error) {
	runtime, ok := snapshot.IR.Runtimes[agent.Runtime]
	if !ok {
		return graph.Runtime{}, fmt.Errorf("agent %q references missing compiled runtime %q", agent.ID, agent.Runtime)
	}
	if runtime.Kind != agentspec.RuntimeKindCLIAgent {
		return graph.Runtime{}, fmt.Errorf("agent %q uses runtime %q with kind %q; only cli_agent external runtimes are executable", agent.ID, runtime.ID, runtime.Kind)
	}
	return runtime, nil
}

func outgoingEdges(snapshot *graph.Snapshot, agentID string) []graph.Edge {
	var outgoing []graph.Edge
	for _, edge := range snapshot.IR.Edges {
		if edge.From == agentID {
			outgoing = append(outgoing, edge)
		}
	}
	return outgoing
}

func handoffRunStatus(first *clirunner.Result, second *clirunner.Result) string {
	if first == nil {
		return clirunner.StatusFailed
	}
	if first.Status != clirunner.StatusCompleted {
		return first.Status
	}
	if second == nil {
		return clirunner.StatusFailed
	}
	return second.Status
}

func appendExternalFailure(ctx context.Context, traceStore *tracepkg.Store, runID string, agentID string, runtimeID string, message string) error {
	if err := traceStore.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventAdapterFailed,
		NodeID:    agentID,
		RuntimeID: runtimeID,
		Payload: jsonPayload(map[string]string{
			"message": sharedcontext.RedactText(limitText(message, previewLimit)),
		}),
	}); err != nil {
		return err
	}
	return traceStore.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventRunFailed,
		NodeID:    agentID,
		RuntimeID: runtimeID,
	})
}

func appendModelFailure(ctx context.Context, traceStore *tracepkg.Store, runID string, agentID string, code string, message string) error {
	if err := traceStore.Append(ctx, &tracepkg.Event{
		RunID:  runID,
		Type:   tracepkg.EventModelFailed,
		NodeID: agentID,
		Payload: jsonPayload(map[string]string{
			"code":    code,
			"message": sharedcontext.RedactText(limitText(message, previewLimit)),
		}),
	}); err != nil {
		return err
	}
	return traceStore.Append(ctx, &tracepkg.Event{
		RunID:  runID,
		Type:   tracepkg.EventRunFailed,
		NodeID: agentID,
	})
}

func appendHandoffCreated(ctx context.Context, traceStore *tracepkg.Store, runID string, fromAgent graph.Agent, toAgent graph.Agent, fromRuntime graph.Runtime, edge graph.Edge, snapshot *sharedcontext.Snapshot) error {
	return traceStore.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventHandoffCreated,
		NodeID:    fromAgent.ID,
		RuntimeID: fromRuntime.ID,
		Payload: jsonPayload(map[string]any{
			"from":        fromAgent.ID,
			"to":          toAgent.ID,
			"edge_id":     edge.ID,
			"mode":        edge.Mode,
			"snapshot_id": snapshot.SnapshotID,
		}),
	})
}

func appendHandoffContextAttached(ctx context.Context, traceStore *tracepkg.Store, runID string, fromAgent graph.Agent, toAgent graph.Agent, toRuntime graph.Runtime, snapshot *sharedcontext.Snapshot) error {
	return traceStore.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventHandoffContextAttached,
		NodeID:    toAgent.ID,
		RuntimeID: toRuntime.ID,
		Payload: jsonPayload(map[string]any{
			"from":        fromAgent.ID,
			"to":          toAgent.ID,
			"snapshot_id": snapshot.SnapshotID,
		}),
	})
}

func appendPolicyTrace(ctx context.Context, traceStore *tracepkg.Store, runID string, agentID string, runtimeID string, action policy.ActionRequest, decision *policy.Decision) error {
	payload := jsonPayload(map[string]any{
		"decision":    decision.Decision,
		"risk":        decision.Risk,
		"reason":      decision.Reason,
		"approval_id": decision.ApprovalID,
		"scope":       decision.Scope,
		"action_type": action.ActionType,
		"summary":     action.Summary,
		"workspace":   action.Workspace,
		"fingerprint": decision.Fingerprint,
		"files_write": action.FilesWrite,
	})
	if err := traceStore.Append(ctx, &tracepkg.Event{
		RunID:     runID,
		Type:      tracepkg.EventPolicyChecked,
		NodeID:    agentID,
		RuntimeID: runtimeID,
		Payload:   payload,
	}); err != nil {
		return err
	}
	switch decision.Decision {
	case policy.DecisionApproval:
		if err := traceStore.Append(ctx, &tracepkg.Event{RunID: runID, Type: tracepkg.EventApprovalRequested, NodeID: agentID, RuntimeID: runtimeID, Payload: payload}); err != nil {
			return err
		}
		return traceStore.Append(ctx, &tracepkg.Event{RunID: runID, Type: tracepkg.EventPolicyBlocked, NodeID: agentID, RuntimeID: runtimeID, Payload: payload})
	case policy.DecisionDeny:
		return traceStore.Append(ctx, &tracepkg.Event{RunID: runID, Type: tracepkg.EventPolicyBlocked, NodeID: agentID, RuntimeID: runtimeID, Payload: payload})
	default:
		return nil
	}
}

func policyBlockedError(decision *policy.Decision) error {
	switch decision.Decision {
	case policy.DecisionApproval:
		return fmt.Errorf("approval required: %s. Approval: %s", decision.Reason, decision.ApprovalID)
	case policy.DecisionDeny:
		return fmt.Errorf("policy denied action: %s", decision.Reason)
	default:
		return fmt.Errorf("policy blocked action: %s", decision.Reason)
	}
}

func policySummary(runtime graph.Runtime, agent graph.Agent, workspace string) string {
	if RuntimeFilesWrite(runtime) {
		return fmt.Sprintf("Run mutable cli_agent %s for agent %s in %s", runtime.ID, agent.ID, workspace)
	}
	return fmt.Sprintf("Run read-only cli_agent %s for agent %s in %s", runtime.ID, agent.ID, workspace)
}

func cliCompletionPayload(result *clirunner.Result) map[string]any {
	payload := map[string]any{
		"status":         result.Status,
		"exit_code":      result.ExitCode,
		"changed_files":  result.ChangedFiles,
		"stdout_ref":     result.StdoutRef,
		"stderr_ref":     result.StderrRef,
		"diff_ref":       result.DiffRef,
		"stdout_preview": sharedcontext.RedactText(limitText(result.Stdout, previewLimit)),
		"stderr_preview": sharedcontext.RedactText(limitText(result.Stderr, previewLimit)),
	}
	if result.Error != "" {
		payload["error"] = sharedcontext.RedactText(limitText(result.Error, previewLimit))
	}
	return payload
}

func cliArtifacts(result *clirunner.Result) []map[string]string {
	artifacts := []map[string]string{}
	add := func(kind string, path string) {
		if path == "" {
			return
		}
		artifacts = append(artifacts, map[string]string{"kind": kind, "path": path})
	}
	add("stdout", result.StdoutRef)
	add("stderr", result.StderrRef)
	add("pre_diff", result.PreDiffRef)
	add("diff", result.DiffRef)
	return artifacts
}

func contextSnapshotPayload(snapshot *sharedcontext.Snapshot, kind string) map[string]any {
	return map[string]any{
		"snapshot_id":   snapshot.SnapshotID,
		"kind":          kind,
		"from_agent":    snapshot.FromAgent,
		"to_agent":      snapshot.ToAgent,
		"summary":       snapshot.Summary,
		"artifact_refs": snapshot.ArtifactRefs,
	}
}

func cliContextSummary(result *clirunner.Result) string {
	if result == nil {
		return "No CLI result was produced."
	}
	if result.ContextSnapshot != nil && strings.TrimSpace(result.ContextSnapshot.Summary) != "" {
		return sharedcontext.RedactText(result.ContextSnapshot.Summary)
	}
	if strings.TrimSpace(result.Stdout) != "" {
		return oneLine(sharedcontext.RedactText(result.Stdout), 1000)
	}
	if result.Error != "" {
		return sharedcontext.RedactText(result.Error)
	}
	if len(result.ChangedFiles) > 0 {
		return "Changed files: " + strings.Join(result.ChangedFiles, ", ")
	}
	return "CLI agent completed without stdout."
}

func cliContextDecisions(result *clirunner.Result) []sharedcontext.Note {
	if result == nil || result.ContextSnapshot == nil {
		return nil
	}
	decisions := make([]sharedcontext.Note, 0, len(result.ContextSnapshot.Decisions))
	for _, decision := range result.ContextSnapshot.Decisions {
		decisions = append(decisions, sharedcontext.Note{Title: sharedcontext.RedactText(decision.Title), Body: sharedcontext.RedactText(decision.Body)})
	}
	return decisions
}

func cliOpenIssues(result *clirunner.Result) []string {
	if result == nil {
		return nil
	}
	if result.ContextSnapshot != nil && len(result.ContextSnapshot.OpenIssues) > 0 {
		openIssues := make([]string, 0, len(result.ContextSnapshot.OpenIssues))
		for _, issue := range result.ContextSnapshot.OpenIssues {
			openIssues = append(openIssues, sharedcontext.RedactText(issue))
		}
		return openIssues
	}
	if result.Status == clirunner.StatusCompleted {
		return nil
	}
	if result.Error == "" {
		return []string{"Upstream CLI agent failed without a structured error."}
	}
	return []string{sharedcontext.RedactText(result.Error)}
}

func cliRecommendations(toAgent string, result *clirunner.Result) []string {
	if result == nil {
		return nil
	}
	if result.ContextSnapshot != nil && len(result.ContextSnapshot.Recommendations) > 0 {
		recommendations := make([]string, 0, len(result.ContextSnapshot.Recommendations))
		for _, recommendation := range result.ContextSnapshot.Recommendations {
			recommendations = append(recommendations, sharedcontext.RedactText(recommendation))
		}
		return recommendations
	}
	if toAgent == "" || result.Status != clirunner.StatusCompleted {
		return nil
	}
	return []string{"Use the upstream summary and artifacts as the handoff context."}
}

func cliArtifactRefs(result *clirunner.Result) []string {
	if result == nil {
		return nil
	}
	refs := []string{}
	if result.ContextSnapshot != nil {
		for _, ref := range result.ContextSnapshot.ArtifactRefs {
			if ref != "" {
				refs = append(refs, sharedcontext.RedactText(ref))
			}
		}
	}
	for _, artifact := range cliArtifacts(result) {
		if path := artifact["path"]; path != "" {
			refs = append(refs, path)
		}
	}
	return refs
}

func redactMessages(messages []adapters.Message) []adapters.Message {
	redacted := make([]adapters.Message, 0, len(messages))
	for _, message := range messages {
		redacted = append(redacted, adapters.Message{Role: message.Role, Content: sharedcontext.RedactText(limitText(message.Content, previewLimit))})
	}
	return redacted
}

func messagePreview(messages []adapters.Message) string {
	if len(messages) == 0 {
		return ""
	}
	return sharedcontext.RedactText(limitText(messages[0].Content, previewLimit))
}

func limitText(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}

func oneLine(value string, max int) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
	return limitText(value, max)
}

func jsonPayload(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("{}")
	}
	return payload
}

func ApprovalTraceEvent(ctx context.Context, traceStore *tracepkg.Store, eventType string, approval *policy.Approval) error {
	if traceStore == nil || approval == nil {
		return nil
	}
	payload := map[string]any{
		"approval_id": approval.ApprovalID,
		"risk":        approval.Risk,
		"summary":     approval.Summary,
	}
	if approval.Scope != "" {
		payload["scope"] = approval.Scope
	}
	return traceStore.Append(ctx, &tracepkg.Event{
		RunID:     approval.RunID,
		Type:      eventType,
		NodeID:    approval.RequestedByAgent,
		RuntimeID: approval.RuntimeID,
		Payload:   jsonPayload(payload),
	})
}

func DBExecutor(db *sql.DB, adapter *adapters.OpenAICompatibleAdapter, resolver *secrets.Resolver, configPath string) *Executor {
	return &Executor{
		Providers:  providers.NewStore(db),
		Trace:      tracepkg.NewStore(db),
		Secrets:    resolver,
		Adapter:    adapter,
		Policy:     policy.NewService(db),
		Context:    sharedcontext.NewStore(db),
		ConfigPath: configPath,
	}
}
