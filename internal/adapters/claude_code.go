package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"time"
)

const defaultClaudeCodeTimeout = 10 * time.Minute

type ClaudeCodeAdapter struct {
	Executable string
}

func NewClaudeCodeAdapter() *ClaudeCodeAdapter {
	return &ClaudeCodeAdapter{Executable: "claude"}
}

func (adapter *ClaudeCodeAdapter) Invoke(ctx context.Context, model string, request InvokeRequest) (*InvokeResult, error) {
	executable := strings.TrimSpace(adapter.Executable)
	if executable == "" {
		executable = "claude"
	}
	if _, err := exec.LookPath(executable); err != nil {
		return &InvokeResult{
			Status: StatusFailed,
			Error:  &AdapterError{Code: ErrorExecutableUnavailable, Message: "claude executable was not found on PATH", Retryable: false},
		}, nil
	}
	timeout := defaultClaudeCodeTimeout
	if request.Options.TimeoutMs > 0 {
		timeout = time.Duration(request.Options.TimeoutMs) * time.Millisecond
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{
		"-p",
		renderCodexPrompt(request.Messages),
		"--output-format", "json",
		"--no-session-persistence",
		"--tools", "",
	}
	if strings.TrimSpace(model) != "" {
		args = append(args, "--model", model)
	}
	command := exec.CommandContext(runCtx, executable, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	runErr := command.Run()
	if runCtx.Err() != nil {
		return &InvokeResult{
			Status: StatusFailed,
			Error:  &AdapterError{Code: ErrorTimeout, Message: "Claude Code invocation timed out", Retryable: true},
			RawRef: limitOutput(stderr.String()),
		}, nil
	}
	if runErr != nil {
		return &InvokeResult{
			Status: StatusFailed,
			Error:  claudeCodeError(runErr, stderr.String()),
			RawRef: limitOutput(stdout.String() + stderr.String()),
		}, nil
	}
	content := parseClaudeCodeOutput(stdout.String())
	return &InvokeResult{
		Status:   StatusCompleted,
		Messages: []Message{{Role: "assistant", Content: content}},
	}, nil
}

func parseClaudeCodeOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	var decoded struct {
		Result string `json:"result"`
		Type   string `json:"type"`
	}
	if err := json.Unmarshal([]byte(output), &decoded); err == nil && strings.TrimSpace(decoded.Result) != "" {
		return strings.TrimSpace(decoded.Result)
	}
	return output
}

func claudeCodeError(err error, stderr string) *AdapterError {
	message := strings.TrimSpace(stderr)
	if message == "" {
		message = err.Error()
	}
	code := ErrorExecutionFailed
	lower := strings.ToLower(message)
	if strings.Contains(lower, "auth") || strings.Contains(lower, "login") || strings.Contains(lower, "oauth") {
		code = ErrorAuthFailed
	}
	var exitErr *exec.ExitError
	retryable := !errors.As(err, &exitErr)
	return &AdapterError{Code: code, Message: limitOutput(message), Retryable: retryable}
}
