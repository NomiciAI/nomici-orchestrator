package toolbroker

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	artifactpkg "github.com/NomiciAI/nomici-orchestrator/internal/artifacts"
	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
	"github.com/NomiciAI/nomici-orchestrator/internal/policy"
	runpkg "github.com/NomiciAI/nomici-orchestrator/internal/runs"
	"github.com/NomiciAI/nomici-orchestrator/internal/sandbox"
	"github.com/NomiciAI/nomici-orchestrator/internal/sharedcontext"
	"github.com/NomiciAI/nomici-orchestrator/internal/trace"
	"github.com/NomiciAI/nomici-orchestrator/internal/worklocks"
)

type Broker struct {
	Store      *Store
	Policy     *policy.Service
	Trace      *trace.Store
	Runs       *runpkg.Store
	Sandboxes  *sandbox.Store
	Artifacts  *artifactpkg.Store
	Locks      *worklocks.Store
	ConfigPath string
	HTTPClient *http.Client
}

func (broker *Broker) Execute(ctx context.Context, request ExecuteRequest) (*CallRecord, error) {
	definition, err := DefinitionByID(strings.TrimSpace(request.ToolID))
	if err != nil {
		return nil, err
	}
	if broker.Store == nil {
		return nil, fmt.Errorf("tool broker store is not initialized")
	}
	session, err := broker.session(ctx, request)
	if err != nil {
		return nil, err
	}
	request.SessionID = session.SessionID
	request.RunID = session.RunID
	inputPreview := previewJSON(request.Input)
	risk := definition.MutationRisk
	if definition.ID == ToolBash {
		risk = ClassifyBashRisk(stringInput(request.Input, "command", "")).Level
	}
	record, err := broker.Store.Create(ctx, CreateCallRequest{
		SessionID:    request.SessionID,
		RunID:        request.RunID,
		TaskID:       request.TaskID,
		ToolID:       definition.ID,
		Status:       StatusPending,
		Risk:         risk,
		InputPreview: inputPreview,
		Metadata:     jsonPayload(map[string]any{"definition": definition}),
	})
	if err != nil {
		return nil, err
	}
	_ = broker.appendTrace(ctx, request.RunID, "tool.call.created", request.AgentID, map[string]any{
		"tool_call_id":  record.ToolCallID,
		"tool_id":       definition.ID,
		"task_id":       request.TaskID,
		"input_preview": sharedcontext.RedactText(inputPreview),
	})
	if definition.ID == ToolBash && risk == RiskCritical {
		message := "command blocked by safety policy: " + ClassifyBashRisk(stringInput(request.Input, "command", "")).Reason
		updated, _ := broker.Store.MarkFailed(ctx, record.ToolCallID, "risk: critical", message, []string{"stdout_stderr_preview"})
		_ = broker.appendTrace(ctx, request.RunID, trace.EventPolicyBlocked, request.AgentID, map[string]any{
			"tool_call_id": record.ToolCallID,
			"tool_id":      definition.ID,
			"risk":         risk,
			"reason":       message,
		})
		return updated, errors.New(message)
	}

	if RequiresApproval(definition.ID) {
		decision, err := broker.checkPolicy(ctx, session, request, definition, record)
		if err != nil {
			updated, _ := broker.Store.MarkFailed(ctx, record.ToolCallID, "", err.Error(), nil)
			return updated, err
		}
		if risk == RiskCritical {
			decision.Risk = RiskCritical
		}
		_ = broker.appendTrace(ctx, request.RunID, trace.EventPolicyChecked, request.AgentID, map[string]any{
			"tool_call_id": record.ToolCallID,
			"tool_id":      definition.ID,
			"decision":     decision.Decision,
			"risk":         decision.Risk,
			"approval_id":  decision.ApprovalID,
			"reason":       decision.Reason,
		})
		switch decision.Decision {
		case policy.DecisionDeny:
			updated, _ := broker.Store.update(ctx, record.ToolCallID, StatusDenied, decision.Risk, "", decision.ApprovalID, decision.Reason, nil, nil)
			_ = broker.appendTrace(ctx, request.RunID, trace.EventPolicyBlocked, request.AgentID, map[string]any{
				"tool_call_id": record.ToolCallID,
				"tool_id":      definition.ID,
				"reason":       decision.Reason,
			})
			return updated, fmt.Errorf("tool call denied: %s", decision.Reason)
		case policy.DecisionApproval:
			updated, err := broker.Store.MarkWaitingApproval(ctx, record.ToolCallID, decision.Risk, decision.ApprovalID, decision.Reason)
			if err != nil {
				return nil, err
			}
			_ = broker.appendTrace(ctx, request.RunID, trace.EventApprovalRequested, request.AgentID, map[string]any{
				"tool_call_id": record.ToolCallID,
				"tool_id":      definition.ID,
				"approval_id":  decision.ApprovalID,
				"risk":         decision.Risk,
				"reason":       decision.Reason,
			})
			return updated, nil
		}
		record.Risk = decision.Risk
	}

	if _, err := broker.Store.MarkRunning(ctx, record.ToolCallID, record.Risk); err != nil {
		return nil, err
	}
	output, artifactRefs, redactions, execErr := broker.executeAllowed(ctx, session, request, definition, record.ToolCallID)
	if execErr != nil {
		updated, _ := broker.Store.MarkFailed(ctx, record.ToolCallID, output, execErr.Error(), redactions)
		_ = broker.appendTrace(ctx, request.RunID, "tool.call.failed", request.AgentID, map[string]any{
			"tool_call_id":   record.ToolCallID,
			"tool_id":        definition.ID,
			"error":          execErr.Error(),
			"output_preview": sharedcontext.RedactText(output),
		})
		return updated, execErr
	}
	updated, err := broker.Store.MarkCompleted(ctx, record.ToolCallID, output, artifactRefs, redactions)
	if err != nil {
		return nil, err
	}
	_ = broker.appendTrace(ctx, request.RunID, "tool.call.completed", request.AgentID, map[string]any{
		"tool_call_id":   record.ToolCallID,
		"tool_id":        definition.ID,
		"output_preview": sharedcontext.RedactText(output),
		"artifact_refs":  artifactRefs,
	})
	return updated, nil
}

func (broker *Broker) ExecuteApprovedRecord(ctx context.Context, toolCallID string, request ExecuteRequest) (*CallRecord, error) {
	if broker.Store == nil {
		return nil, fmt.Errorf("tool broker store is not initialized")
	}
	record, err := broker.Store.Get(ctx, toolCallID)
	if err != nil {
		return nil, err
	}
	if record.Status == StatusCompleted {
		return record, nil
	}
	if record.Status != StatusWaitingApproval {
		return record, fmt.Errorf("tool call %s is %s, not waiting for approval", toolCallID, record.Status)
	}
	if record.ApprovalID != "" && broker.Policy != nil {
		approval, err := broker.Policy.Get(ctx, record.ApprovalID)
		if err != nil {
			return record, fmt.Errorf("load approval %s: %w", record.ApprovalID, err)
		}
		if approval.Status != policy.StatusGranted {
			return record, fmt.Errorf("approval %s is %s", record.ApprovalID, approval.Status)
		}
	}
	definition, err := DefinitionByID(record.ToolID)
	if err != nil {
		return record, err
	}
	session, err := broker.session(ctx, ExecuteRequest{SessionID: record.SessionID, RunID: record.RunID})
	if err != nil {
		return record, err
	}
	request.SessionID = record.SessionID
	request.RunID = record.RunID
	if request.TaskID == "" {
		request.TaskID = record.TaskID
	}
	if request.ToolID == "" {
		request.ToolID = record.ToolID
	}
	if _, err := broker.Store.MarkRunning(ctx, record.ToolCallID, record.Risk); err != nil {
		return nil, err
	}
	output, artifactRefs, redactions, execErr := broker.executeAllowed(ctx, session, request, definition, record.ToolCallID)
	if execErr != nil {
		updated, _ := broker.Store.MarkFailed(ctx, record.ToolCallID, output, execErr.Error(), redactions)
		_ = broker.appendTrace(ctx, record.RunID, "tool.call.failed", request.AgentID, map[string]any{
			"tool_call_id":   record.ToolCallID,
			"tool_id":        definition.ID,
			"error":          execErr.Error(),
			"output_preview": sharedcontext.RedactText(output),
		})
		return updated, execErr
	}
	updated, err := broker.Store.MarkCompleted(ctx, record.ToolCallID, output, artifactRefs, redactions)
	if err != nil {
		return nil, err
	}
	_ = broker.appendTrace(ctx, record.RunID, "tool.call.completed", request.AgentID, map[string]any{
		"tool_call_id":   record.ToolCallID,
		"tool_id":        definition.ID,
		"output_preview": sharedcontext.RedactText(output),
		"artifact_refs":  artifactRefs,
	})
	return updated, nil
}

func (broker *Broker) session(ctx context.Context, request ExecuteRequest) (*runpkg.Session, error) {
	if broker.Runs == nil {
		if request.SessionID == "" || request.RunID == "" {
			return nil, fmt.Errorf("session_id and run_id are required")
		}
		return &runpkg.Session{SessionID: request.SessionID, RunID: request.RunID}, nil
	}
	if request.SessionID != "" {
		detail, err := broker.Runs.GetBySession(ctx, request.SessionID)
		if err != nil {
			return nil, err
		}
		return detail.Session, nil
	}
	if request.RunID != "" {
		detail, err := broker.Runs.GetByRun(ctx, request.RunID)
		if err != nil {
			return nil, err
		}
		return detail.Session, nil
	}
	return nil, fmt.Errorf("session_id or run_id is required")
}

func (broker *Broker) checkPolicy(ctx context.Context, session *runpkg.Session, request ExecuteRequest, definition Definition, record *CallRecord) (*policy.Decision, error) {
	if broker.Policy == nil {
		return nil, fmt.Errorf("policy service is not initialized")
	}
	workspace := ""
	if sandboxRecord, err := broker.sandbox(ctx, session.RunID); err == nil {
		workspace = sandboxRecord.WorkspaceRoot
	}
	action := policy.ActionRequest{
		RunID:      session.RunID,
		ActionID:   ids.New("action"),
		ActionType: "tool." + definition.ID,
		ProjectID:  session.ProjectID,
		AgentID:    request.AgentID,
		Workspace:  workspace,
		FilesWrite: definition.FilesystemRisk == "write" || definition.FilesystemRisk == "read_write",
		Summary:    definition.Description,
		Subject: map[string]string{
			"tool_call_id": record.ToolCallID,
			"tool_id":      definition.ID,
			"task_id":      request.TaskID,
			"workspace":    workspace,
		},
	}
	return broker.Policy.Check(ctx, action)
}

func (broker *Broker) executeAllowed(ctx context.Context, session *runpkg.Session, request ExecuteRequest, definition Definition, toolCallID string) (string, []string, []string, error) {
	switch definition.ID {
	case ToolListFiles:
		return broker.listFiles(ctx, session.RunID, request.Input)
	case ToolReadFile:
		return broker.readFile(ctx, session.RunID, request.Input)
	case ToolWriteFile:
		if err := broker.requireSandboxPermission(ctx, session.RunID, "file_write_enabled"); err != nil {
			return "", nil, nil, err
		}
		return broker.writeFile(ctx, session.RunID, request.TaskID, toolCallID, request.Input)
	case ToolReplaceFile:
		if err := broker.requireSandboxPermission(ctx, session.RunID, "file_write_enabled"); err != nil {
			return "", nil, nil, err
		}
		return broker.replaceFile(ctx, session.RunID, request.TaskID, toolCallID, request.Input)
	case ToolPresentArtifact:
		if err := broker.requireSandboxPermission(ctx, session.RunID, "file_write_enabled"); err != nil {
			return "", nil, nil, err
		}
		return broker.presentArtifact(ctx, session, request, toolCallID, request.Input)
	case ToolBash:
		if err := broker.requireSandboxPermission(ctx, session.RunID, "bash_enabled"); err != nil {
			return "", nil, nil, err
		}
		return broker.runBash(ctx, session.RunID, request.TaskID, toolCallID, request.Input)
	case ToolSearch:
		return broker.search(ctx, request.Input)
	case ToolFetch:
		return broker.fetch(ctx, request.Input)
	default:
		return "", nil, nil, fmt.Errorf("tool %q is not executable", definition.ID)
	}
}

func (broker *Broker) requireSandboxPermission(ctx context.Context, runID string, key string) error {
	record, err := broker.sandbox(ctx, runID)
	if err != nil {
		return err
	}
	metadata := map[string]any{}
	if len(record.Metadata) > 0 {
		_ = json.Unmarshal(record.Metadata, &metadata)
	}
	enabled, _ := metadata[key].(bool)
	if !enabled {
		return fmt.Errorf("sandbox policy does not enable %s", key)
	}
	return nil
}

func (broker *Broker) listFiles(ctx context.Context, runID string, input map[string]any) (string, []string, []string, error) {
	root, err := broker.workspacePath(ctx, runID, stringInput(input, "path", "."))
	if err != nil {
		return "", nil, nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", nil, nil, err
	}
	limit := intInput(input, "limit", 100)
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	lines := []string{}
	for index, entry := range entries {
		if index >= limit {
			lines = append(lines, fmt.Sprintf("... %d more", len(entries)-limit))
			break
		}
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		lines = append(lines, name)
	}
	return strings.Join(lines, "\n"), nil, []string{"paths_only"}, nil
}

func (broker *Broker) readFile(ctx context.Context, runID string, input map[string]any) (string, []string, []string, error) {
	path, err := broker.workspacePath(ctx, runID, stringInput(input, "path", ""))
	if err != nil {
		return "", nil, nil, err
	}
	maxBytes := intInput(input, "max_bytes", 20000)
	if maxBytes <= 0 || maxBytes > 200000 {
		maxBytes = 20000
	}
	file, err := os.Open(path)
	if err != nil {
		return "", nil, nil, err
	}
	defer file.Close()
	var buffer bytes.Buffer
	written, err := io.CopyN(&buffer, file, int64(maxBytes)+1)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", nil, nil, err
	}
	output := buffer.String()
	if written > int64(maxBytes) {
		output = output[:maxBytes] + "\n... truncated"
	}
	return output, nil, []string{"content_preview"}, nil
}

func (broker *Broker) writeFile(ctx context.Context, runID string, taskID string, toolCallID string, input map[string]any) (string, []string, []string, error) {
	path, err := broker.workspacePath(ctx, runID, stringInput(input, "path", ""))
	if err != nil {
		return "", nil, nil, err
	}
	content, ok := input["content"].(string)
	if !ok {
		return "", nil, nil, fmt.Errorf("content is required")
	}
	var output string
	err = broker.withWorkspaceLock(ctx, runID, taskID, toolCallID, "file:"+path, func() error {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return err
		}
		output = "wrote " + relativeDisplay(path)
		return nil
	})
	if err != nil {
		return "", nil, nil, err
	}
	return output, nil, []string{"content_preview"}, nil
}

func (broker *Broker) replaceFile(ctx context.Context, runID string, taskID string, toolCallID string, input map[string]any) (string, []string, []string, error) {
	path, err := broker.workspacePath(ctx, runID, stringInput(input, "path", ""))
	if err != nil {
		return "", nil, nil, err
	}
	oldText, _ := input["old"].(string)
	newText, _ := input["new"].(string)
	if oldText == "" {
		return "", nil, nil, fmt.Errorf("old text is required")
	}
	var output string
	err = broker.withWorkspaceLock(ctx, runID, taskID, toolCallID, "file:"+path, func() error {
		payload, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(payload)
		count := strings.Count(content, oldText)
		if count == 0 {
			return fmt.Errorf("old text was not found")
		}
		replaceAll, _ := input["all"].(bool)
		if replaceAll {
			content = strings.ReplaceAll(content, oldText, newText)
		} else {
			content = strings.Replace(content, oldText, newText, 1)
			count = 1
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return err
		}
		output = fmt.Sprintf("replaced %d occurrence(s) in %s", count, relativeDisplay(path))
		return nil
	})
	if err != nil {
		return "", nil, nil, err
	}
	return output, nil, []string{"content_preview"}, nil
}

func (broker *Broker) presentArtifact(ctx context.Context, session *runpkg.Session, request ExecuteRequest, toolCallID string, input map[string]any) (string, []string, []string, error) {
	if broker.Artifacts == nil {
		return "", nil, nil, fmt.Errorf("artifact store is not initialized")
	}
	artifactType := stringInput(input, "type", artifactpkg.TypeFile)
	title := stringInput(input, "title", artifactType)
	preview := stringInput(input, "preview", "")
	path := ""
	if rawPath := stringInput(input, "path", ""); rawPath != "" {
		resolved, err := broker.workspacePath(ctx, session.RunID, rawPath)
		if err != nil {
			return "", nil, nil, err
		}
		path = resolved
		if preview == "" {
			payload, _ := os.ReadFile(path)
			preview = trimPreview(string(payload), 4000)
		}
	}
	var artifact *artifactpkg.Artifact
	err := broker.withWorkspaceLock(ctx, session.RunID, request.TaskID, toolCallID, "artifacts", func() error {
		created, err := broker.Artifacts.Create(ctx, artifactpkg.CreateRequest{
			SessionID:   session.SessionID,
			RunID:       session.RunID,
			TaskID:      request.TaskID,
			Type:        artifactType,
			Title:       title,
			Path:        path,
			ReviewState: artifactpkg.ReviewApproved,
			Preview:     preview,
			Metadata:    jsonPayload(map[string]any{"source": "tool_call", "tool_id": ToolPresentArtifact}),
		})
		if err != nil {
			return err
		}
		artifact = created
		return nil
	})
	if err != nil {
		return "", nil, nil, err
	}
	if broker.Runs != nil && request.TaskID != "" {
		_ = broker.Runs.AddTaskArtifactRef(ctx, request.TaskID, artifact.ArtifactID)
	}
	return "artifact created: " + artifact.ArtifactID, []string{artifact.ArtifactID}, []string{"content_preview"}, nil
}

func (broker *Broker) runBash(ctx context.Context, runID string, taskID string, toolCallID string, input map[string]any) (string, []string, []string, error) {
	command := strings.TrimSpace(stringInput(input, "command", ""))
	if command == "" {
		return "", nil, nil, fmt.Errorf("command is required")
	}
	classification := ClassifyBashRisk(command)
	if classification.Level == "critical" {
		return "risk: critical\nreason: " + classification.Reason, nil, []string{"stdout_stderr_preview"}, fmt.Errorf("command blocked by safety policy: %s", classification.Reason)
	}
	cwd, err := broker.workspacePath(ctx, runID, stringInput(input, "cwd", "."))
	if err != nil {
		return "", nil, nil, err
	}
	timeout := intInput(input, "timeout_seconds", 60)
	if timeout <= 0 || timeout > 300 {
		timeout = 60
	}
	var output string
	err = broker.withWorkspaceLock(ctx, runID, taskID, toolCallID, "workspace", func() error {
		var err error
		output, err = broker.runShellCommand(ctx, runID, cwd, command, timeout, classification)
		return err
	})
	if err != nil {
		return output, nil, []string{"stdout_stderr_preview"}, err
	}
	return output, nil, []string{"stdout_stderr_preview"}, nil
}

func (broker *Broker) runShellCommand(ctx context.Context, runID string, cwd string, command string, timeout int, classification BashRisk) (string, error) {
	sandboxRecord, err := broker.sandbox(ctx, runID)
	if err != nil {
		return "", err
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	cmd, err := broker.shellCommand(runCtx, sandboxRecord, cwd, command)
	if err != nil {
		return "", err
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	exitCode := 0
	if err != nil {
		exitCode = -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}
	output := fmt.Sprintf("risk: %s\nrisk_reason: %s\nexit_code: %d\nstdout:\n%s\nstderr:\n%s", classification.Level, classification.Reason, exitCode, trimPreview(stdout.String(), 4000), trimPreview(stderr.String(), 4000))
	if runCtx.Err() == context.DeadlineExceeded {
		return output, fmt.Errorf("command timed out after %d seconds", timeout)
	}
	if err != nil {
		return output, fmt.Errorf("command exited with %d", exitCode)
	}
	return output, nil
}

func (broker *Broker) shellCommand(ctx context.Context, record *sandbox.Record, cwd string, command string) (*exec.Cmd, error) {
	if record != nil && record.Mode == sandbox.ModeContainer {
		return containerShellCommand(ctx, record, cwd, command)
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Dir = cwd
	return cmd, nil
}

func containerShellCommand(ctx context.Context, record *sandbox.Record, cwd string, command string) (*exec.Cmd, error) {
	if record == nil {
		return nil, fmt.Errorf("sandbox record is required")
	}
	binary := strings.TrimSpace(record.RuntimeBinary)
	if binary == "" {
		return nil, fmt.Errorf("container sandbox runtime is not configured")
	}
	name := filepath.Base(binary)
	if name != "docker" && name != "podman" {
		return nil, fmt.Errorf("container bash adapter supports docker or podman, got %s", name)
	}
	root := strings.TrimSpace(record.WorkspaceRoot)
	if root == "" {
		return nil, fmt.Errorf("container sandbox workspace root is not configured")
	}
	rel, err := filepath.Rel(root, cwd)
	if err != nil {
		return nil, err
	}
	if rel == "." {
		rel = ""
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("cwd escapes the workspace")
	}
	image := containerImage(record)
	workspaceCWD := "/workspace"
	if rel != "" {
		workspaceCWD = "/workspace/" + filepath.ToSlash(rel)
	}
	args := []string{
		"run",
		"--rm",
		"-v", root + ":/workspace",
		"-w", workspaceCWD,
		image,
		"/bin/sh",
		"-c",
		command,
	}
	return exec.CommandContext(ctx, binary, args...), nil
}

func containerImage(record *sandbox.Record) string {
	if record == nil || len(record.Metadata) == 0 {
		return "ubuntu:24.04"
	}
	metadata := map[string]any{}
	if err := json.Unmarshal(record.Metadata, &metadata); err != nil {
		return "ubuntu:24.04"
	}
	if image, _ := metadata["container_image"].(string); strings.TrimSpace(image) != "" {
		return strings.TrimSpace(image)
	}
	return "ubuntu:24.04"
}

type BashRisk struct {
	Level  string
	Reason string
}

func ClassifyBashRisk(command string) BashRisk {
	lower := strings.ToLower(strings.TrimSpace(command))
	switch {
	case lower == "":
		return BashRisk{Level: "low", Reason: "empty command"}
	case containsDangerousCommand(lower):
		return BashRisk{Level: "critical", Reason: "command contains destructive host or credential access patterns"}
	case strings.Contains(lower, "sudo ") ||
		strings.Contains(lower, "curl ") && strings.Contains(lower, "| sh") ||
		strings.Contains(lower, "wget ") && strings.Contains(lower, "| sh") ||
		strings.Contains(lower, "chmod -r 777") ||
		strings.Contains(lower, "mkfs."):
		return BashRisk{Level: "critical", Reason: "command can mutate system state outside the workspace"}
	case strings.Contains(lower, "rm ") ||
		strings.Contains(lower, "git push") ||
		strings.Contains(lower, "npm publish") ||
		strings.Contains(lower, "deploy") ||
		strings.Contains(lower, "docker run") ||
		strings.Contains(lower, "podman run"):
		return BashRisk{Level: "high", Reason: "command may mutate files, remote state, or external processes"}
	case strings.Contains(lower, "make test") ||
		strings.Contains(lower, "go test") ||
		strings.Contains(lower, "pnpm test") ||
		strings.Contains(lower, "npm test"):
		return BashRisk{Level: "medium", Reason: "command executes project code for verification"}
	default:
		return BashRisk{Level: "medium", Reason: "shell command execution requires workspace mediation"}
	}
}

func containsDangerousCommand(lower string) bool {
	patterns := []string{
		"rm -rf /",
		"rm -fr /",
		"rm -rf ~",
		"rm -fr ~",
		":(){",
		"dd if=",
		"diskutil erase",
		"security find-generic-password",
		"cat ~/.ssh",
		"cat ~/.aws",
		"cat ~/.config",
		"chmod -r 777 /",
	}
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func (broker *Broker) withWorkspaceLock(ctx context.Context, runID string, taskID string, toolCallID string, resource string, fn func() error) error {
	if broker.Locks == nil {
		return fn()
	}
	lock, err := broker.Locks.Acquire(ctx, worklocks.AcquireRequest{
		RunID:      runID,
		TaskID:     taskID,
		ToolCallID: toolCallID,
		Resource:   resource,
		Owner:      "toolbroker",
		Metadata:   jsonPayload(map[string]any{"tool_call_id": toolCallID}),
	})
	if err != nil {
		return err
	}
	_ = broker.appendTrace(ctx, runID, "workspace.lock.acquired", "", map[string]any{
		"lock_id":      lock.LockID,
		"resource":     lock.Resource,
		"task_id":      taskID,
		"tool_call_id": toolCallID,
	})
	defer func() {
		_ = broker.Locks.Release(ctx, lock.LockID)
		_ = broker.appendTrace(ctx, runID, "workspace.lock.released", "", map[string]any{
			"lock_id":      lock.LockID,
			"resource":     lock.Resource,
			"task_id":      taskID,
			"tool_call_id": toolCallID,
		})
	}()
	return fn()
}

func (broker *Broker) search(ctx context.Context, input map[string]any) (string, []string, []string, error) {
	query := strings.TrimSpace(stringInput(input, "query", ""))
	if query == "" {
		return "", nil, nil, fmt.Errorf("query is required")
	}
	provider := broker.toolProvider("web_search")
	if provider == "none" {
		return "", nil, nil, fmt.Errorf("web search is not configured")
	}
	limit := intInput(input, "limit", 5)
	if limit <= 0 || limit > 10 {
		limit = 5
	}
	if provider != "" && provider != "duckduckgo" {
		return "", nil, nil, fmt.Errorf("web search provider %q is configured but execution adapter is not installed", provider)
	}
	endpoint := "https://duckduckgo.com/html/?q=" + url.QueryEscape(query)
	body, err := broker.httpGet(ctx, endpoint)
	if err != nil {
		return "", nil, nil, err
	}
	results := parseDuckDuckGo(body, limit)
	if len(results) == 0 {
		return "No search results.", nil, []string{"query_preview"}, nil
	}
	return strings.Join(results, "\n"), nil, []string{"query_preview"}, nil
}

func (broker *Broker) fetch(ctx context.Context, input map[string]any) (string, []string, []string, error) {
	rawURL := strings.TrimSpace(stringInput(input, "url", ""))
	if rawURL == "" {
		return "", nil, nil, fmt.Errorf("url is required")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", nil, nil, fmt.Errorf("valid url is required")
	}
	provider := broker.toolProvider("web_fetch")
	if provider == "none" {
		return "", nil, nil, fmt.Errorf("web fetch is not configured")
	}
	target := rawURL
	if provider == "" || provider == "jina_reader" {
		target = "https://r.jina.ai/" + rawURL
	} else if provider != "direct_http" {
		return "", nil, nil, fmt.Errorf("web fetch provider %q is configured but execution adapter is not installed", provider)
	}
	body, err := broker.httpGet(ctx, target)
	if err != nil {
		return "", nil, nil, err
	}
	return trimPreview(body, 12000), nil, []string{"url_preview"}, nil
}

func (broker *Broker) workspacePath(ctx context.Context, runID string, rawPath string) (string, error) {
	if strings.TrimSpace(rawPath) == "" {
		return "", fmt.Errorf("path is required")
	}
	record, err := broker.sandbox(ctx, runID)
	if err != nil {
		return "", err
	}
	root := record.WorkspaceRoot
	if root == "" {
		return "", fmt.Errorf("workspace root is not available")
	}
	if filepath.IsAbs(rawPath) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	clean := filepath.Clean(rawPath)
	if clean == "." {
		clean = ""
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes the workspace")
	}
	resolved := filepath.Join(root, clean)
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absoluteResolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	if absoluteResolved != absoluteRoot && !strings.HasPrefix(absoluteResolved, absoluteRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes the workspace")
	}
	return absoluteResolved, nil
}

func (broker *Broker) sandbox(ctx context.Context, runID string) (*sandbox.Record, error) {
	if broker.Sandboxes == nil {
		return nil, fmt.Errorf("sandbox store is not initialized")
	}
	record, err := broker.Sandboxes.GetByRun(ctx, runID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("sandbox record was not found")
		}
		return nil, err
	}
	return record, nil
}

func (broker *Broker) toolProvider(toolID string) string {
	configPath := broker.ConfigPath
	if configPath == "" {
		configPath = "nomici.yaml"
	}
	loaded, err := agentspec.LoadFileWithLocal(configPath)
	if err != nil || loaded.Spec == nil || loaded.Spec.Tools == nil {
		switch toolID {
		case "web_search":
			return "duckduckgo"
		case "web_fetch":
			return "jina_reader"
		default:
			return ""
		}
	}
	config := loaded.Spec.Tools[toolID]
	provider, _ := config["provider"].(string)
	if provider == "" {
		provider, _ = config["kind"].(string)
	}
	return strings.TrimSpace(strings.ToLower(strings.ReplaceAll(provider, "-", "_")))
}

func (broker *Broker) httpGet(ctx context.Context, target string) (string, error) {
	client := broker.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "Nomici Tool Broker")
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return string(body), fmt.Errorf("provider returned HTTP %d", response.StatusCode)
	}
	return string(body), nil
}

func (broker *Broker) appendTrace(ctx context.Context, runID string, eventType string, agentID string, payload map[string]any) error {
	if broker.Trace == nil || runID == "" {
		return nil
	}
	return broker.Trace.Append(ctx, &trace.Event{
		RunID:   runID,
		Type:    eventType,
		NodeID:  agentID,
		Payload: jsonPayload(payload),
	})
}

func parseDuckDuckGo(body string, limit int) []string {
	pattern := regexp.MustCompile(`<a[^>]+class="result__a"[^>]+href="([^"]+)"[^>]*>(.*?)</a>`)
	matches := pattern.FindAllStringSubmatch(body, limit)
	results := []string{}
	tagPattern := regexp.MustCompile(`<[^>]+>`)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		link := html.UnescapeString(match[1])
		title := html.UnescapeString(tagPattern.ReplaceAllString(match[2], ""))
		results = append(results, strings.TrimSpace(title)+" - "+strings.TrimSpace(link))
	}
	return results
}

func previewJSON(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return trimPreview(string(payload), 2000)
}

func jsonPayload(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("{}")
	}
	return payload
}

func stringInput(input map[string]any, key string, fallback string) string {
	if input == nil {
		return fallback
	}
	value, ok := input[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(typed)
	}
}

func intInput(input map[string]any, key string, fallback int) int {
	if input == nil {
		return fallback
	}
	value, ok := input[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(typed)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func trimPreview(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "\n... truncated"
}

func relativeDisplay(path string) string {
	if path == "" {
		return ""
	}
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, path); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	return path
}
