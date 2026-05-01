package clirunner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultTimeoutSeconds = 1800
	lockFileName          = "cli_agent.lock"
)

func Invoke(ctx context.Context, config Config, request Request) (*Result, error) {
	if strings.TrimSpace(config.Executable) == "" {
		return nil, fmt.Errorf("cli runner executable is required")
	}
	if strings.TrimSpace(request.RunID) == "" {
		return nil, fmt.Errorf("cli runner run_id is required")
	}
	if strings.TrimSpace(request.TaskID) == "" {
		request.TaskID = "task_" + request.RunID
	}

	workspace, err := workspacePath(config.Workspace)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}

	artifactDir := filepath.Join(workspace, ".nomici", "artifacts", request.RunID)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return nil, fmt.Errorf("create artifact directory: %w", err)
	}

	var unlock func()
	if config.FilesWrite {
		unlock, err = lockWorkspace(workspace)
		if err != nil {
			return failedResult(workspace, artifactDir, -1, err.Error()), nil
		}
		defer unlock()
	}

	preSnapshot, preDiff, err := captureWorkspace(workspace)
	if err != nil {
		return nil, err
	}
	preDiffRef := filepath.Join(artifactDir, "pre.diff")
	if err := os.WriteFile(preDiffRef, []byte(preDiff), 0o600); err != nil {
		return nil, fmt.Errorf("write pre diff artifact: %w", err)
	}

	env, err := buildEnv(config)
	if err != nil {
		return failedResult(workspace, artifactDir, -1, err.Error()), nil
	}
	executable, err := exec.LookPath(config.Executable)
	if err != nil {
		return failedResult(workspace, artifactDir, -1, "executable "+quote(config.Executable)+" was not found on PATH"), nil
	}

	timeoutSeconds := config.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = defaultTimeoutSeconds
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	input := renderInput(request)
	args := renderValues(config.Args, templateValues(config, request, input))
	stdin := renderTemplate(config.Stdin, templateValues(config, request, input))

	result := &Result{
		Status:      StatusFailed,
		ExitCode:    -1,
		Workspace:   workspace,
		ArtifactDir: artifactDir,
		PreDiffRef:  preDiffRef,
		StartedAt:   time.Now().UTC(),
	}

	command := exec.CommandContext(runCtx, executable, args...)
	command.Dir = workspace
	command.Env = env
	if stdin != "" {
		command.Stdin = strings.NewReader(stdin)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	runErr := command.Run()
	result.CompletedAt = time.Now().UTC()
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	result.ContextSnapshot = parseContextSnapshot(stdout.Bytes())
	result.StdoutRef = filepath.Join(artifactDir, "stdout.txt")
	result.StderrRef = filepath.Join(artifactDir, "stderr.txt")

	if err := os.WriteFile(result.StdoutRef, stdout.Bytes(), 0o600); err != nil {
		return nil, fmt.Errorf("write stdout artifact: %w", err)
	}
	if err := os.WriteFile(result.StderrRef, stderr.Bytes(), 0o600); err != nil {
		return nil, fmt.Errorf("write stderr artifact: %w", err)
	}

	_, postDiff, changedFiles, err := captureWorkspaceDiff(workspace, preSnapshot)
	if err != nil {
		return nil, err
	}
	result.DiffRef = filepath.Join(artifactDir, "post.diff")
	if err := os.WriteFile(result.DiffRef, []byte(postDiff), 0o600); err != nil {
		return nil, fmt.Errorf("write post diff artifact: %w", err)
	}
	result.ChangedFiles = changedFiles

	var exitError *exec.ExitError
	switch {
	case runCtx.Err() == context.DeadlineExceeded:
		result.Error = fmt.Sprintf("cli agent timed out after %d seconds", timeoutSeconds)
	case runErr == nil:
		result.Status = StatusCompleted
		result.ExitCode = 0
	case errors.As(runErr, &exitError):
		result.ExitCode = exitError.ExitCode()
		result.Error = fmt.Sprintf("cli agent exited with code %d", result.ExitCode)
	default:
		result.Error = runErr.Error()
	}

	return result, nil
}

type outputEnvelope struct {
	ContextSnapshot *ContextSnapshotCandidate `json:"context_snapshot"`
}

func parseContextSnapshot(stdout []byte) *ContextSnapshotCandidate {
	trimmed := bytes.TrimSpace(stdout)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil
	}
	var envelope outputEnvelope
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return nil
	}
	if envelope.ContextSnapshot == nil || strings.TrimSpace(envelope.ContextSnapshot.Summary) == "" {
		return nil
	}
	return envelope.ContextSnapshot
}

func failedResult(workspace string, artifactDir string, exitCode int, message string) *Result {
	now := time.Now().UTC()
	return &Result{
		Status:      StatusFailed,
		ExitCode:    exitCode,
		Workspace:   workspace,
		ArtifactDir: artifactDir,
		Error:       message,
		StartedAt:   now,
		CompletedAt: now,
	}
}

func workspacePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		path = "."
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve workspace path: %w", err)
	}
	return absolute, nil
}

func lockWorkspace(workspace string) (func(), error) {
	lockDir := filepath.Join(workspace, ".nomici", "locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace lock directory: %w", err)
	}
	lockPath := filepath.Join(lockDir, lockFileName)
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("workspace is locked by another mutable cli_agent run: %s", lockPath)
		}
		return nil, fmt.Errorf("create workspace lock: %w", err)
	}
	_, _ = fmt.Fprintf(file, "pid=%d\ncreated_at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
	_ = file.Close()
	return func() {
		_ = os.Remove(lockPath)
	}, nil
}

func buildEnv(config Config) ([]string, error) {
	values := map[string]string{}
	for _, key := range []string{"PATH", "HOME", "TMPDIR", "TMP", "TEMP", "USER", "USERNAME", "SHELL", "SYSTEMROOT", "COMSPEC"} {
		if value, ok := os.LookupEnv(key); ok {
			values[key] = value
		}
	}
	for key, value := range config.Env {
		if key == "" {
			continue
		}
		values[key] = value
	}
	for _, key := range config.EnvFrom {
		if strings.TrimSpace(key) == "" {
			continue
		}
		value, ok := os.LookupEnv(key)
		if !ok {
			return nil, fmt.Errorf("environment variable %s is required by cli_agent runtime but is not set", key)
		}
		values[key] = value
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+values[key])
	}
	return env, nil
}

func renderInput(request Request) string {
	briefing := strings.TrimSpace(request.Briefing)
	sharedBriefing := strings.TrimSpace(request.SharedContext.Briefing)
	if sharedBriefing != "" {
		if briefing != "" {
			briefing += "\n\n"
		}
		briefing += sharedBriefing
	}
	if briefing == "" {
		return request.Prompt
	}
	if strings.TrimSpace(request.Prompt) == "" {
		return briefing
	}
	return briefing + "\n\nTask:\n" + request.Prompt
}

func templateValues(config Config, request Request, input string) map[string]string {
	return map[string]string{
		"INPUT":     input,
		"PROMPT":    request.Prompt,
		"BRIEFING":  request.Briefing,
		"RUN_ID":    request.RunID,
		"TASK_ID":   request.TaskID,
		"WORKSPACE": config.Workspace,
	}
}

func renderValues(values []string, replacements map[string]string) []string {
	rendered := make([]string, 0, len(values))
	for _, value := range values {
		rendered = append(rendered, renderTemplate(value, replacements))
	}
	return rendered
}

func renderTemplate(value string, replacements map[string]string) string {
	for key, replacement := range replacements {
		value = strings.ReplaceAll(value, "${"+key+"}", replacement)
	}
	return value
}

func hashFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}

func quote(value string) string {
	return `"` + value + `"`
}
