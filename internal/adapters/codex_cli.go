package adapters

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
)

const defaultCodexCLITimeout = 10 * time.Minute

type CodexCLIAdapter struct {
	Executable string
}

func NewCodexCLIAdapter() *CodexCLIAdapter {
	return &CodexCLIAdapter{Executable: "codex"}
}

func (adapter *CodexCLIAdapter) Invoke(ctx context.Context, model string, request InvokeRequest) (*InvokeResult, error) {
	executable, errResult := adapter.resolveExecutable()
	if errResult != nil {
		return errResult, nil
	}

	if !executableIsRunnable(executable) {
		return &InvokeResult{
			Status: StatusFailed,
			Error:  &AdapterError{Code: ErrorExecutableUnavailable, Message: "codex executable is not runnable: " + executable, Retryable: false},
		}, nil
	}

	timeout := defaultCodexCLITimeout
	if request.Options.TimeoutMs > 0 {
		timeout = time.Duration(request.Options.TimeoutMs) * time.Millisecond
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	outputFile, err := os.CreateTemp("", "nomici-codex-last-message-*.txt")
	if err != nil {
		return nil, fmt.Errorf("create Codex CLI output file: %w", err)
	}
	outputPath := outputFile.Name()
	_ = outputFile.Close()
	defer os.Remove(outputPath)

	args := []string{
		"exec",
		"--model", model,
		"--sandbox", "read-only",
		"--ask-for-approval", "never",
		"--skip-git-repo-check",
		"--ephemeral",
		"--color", "never",
		"--output-last-message", outputPath,
		"-",
	}
	command := exec.CommandContext(runCtx, executable, args...)
	command.Stdin = strings.NewReader(renderCodexPrompt(request.Messages))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	runErr := command.Run()
	if runCtx.Err() != nil {
		return &InvokeResult{
			Status: StatusFailed,
			Error:  &AdapterError{Code: ErrorTimeout, Message: "Codex CLI invocation timed out", Retryable: true},
			RawRef: limitOutput(stderr.String()),
		}, nil
	}
	if runErr != nil {
		return &InvokeResult{
			Status: StatusFailed,
			Error:  codexCLIError(runErr, stderr.String()),
			RawRef: limitOutput(stdout.String() + stderr.String()),
		}, nil
	}

	contentBytes, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read Codex CLI output file: %w", err)
	}
	content := strings.TrimSpace(string(contentBytes))
	if content == "" {
		content = strings.TrimSpace(stdout.String())
	}
	return &InvokeResult{
		Status:   StatusCompleted,
		Messages: []Message{{Role: "assistant", Content: content}},
	}, nil
}

func (adapter *CodexCLIAdapter) resolveExecutable() (string, *InvokeResult) {
	configured := strings.TrimSpace(adapter.Executable)
	if configured == "" || configured == "codex" {
		availability := providers.DetectCodexCLI()
		if availability.Available {
			return availability.Executable, nil
		}
		code := ErrorExecutableUnavailable
		if availability.Executable != "" {
			code = ErrorAuthFailed
		}
		return "", &InvokeResult{
			Status: StatusFailed,
			Error:  &AdapterError{Code: code, Message: availability.Message, Retryable: false},
		}
	}
	return configured, nil
}

func executableIsRunnable(executable string) bool {
	if strings.ContainsRune(executable, os.PathSeparator) {
		info, err := os.Stat(executable)
		return err == nil && !info.IsDir()
	}
	_, err := exec.LookPath(executable)
	return err == nil
}

func renderCodexPrompt(messages []Message) string {
	var builder strings.Builder
	for _, message := range messages {
		role := strings.TrimSpace(message.Role)
		if role == "" {
			role = "user"
		}
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(titleRole(role))
		builder.WriteString(":\n")
		builder.WriteString(content)
	}
	return builder.String()
}

func titleRole(role string) string {
	if role == "" {
		return "User"
	}
	return strings.ToUpper(role[:1]) + role[1:]
}

func codexCLIError(err error, stderr string) *AdapterError {
	message := strings.TrimSpace(stderr)
	if message == "" {
		message = err.Error()
	}
	code := ErrorExecutionFailed
	if strings.Contains(strings.ToLower(message), "auth") || strings.Contains(strings.ToLower(message), "login") {
		code = ErrorAuthFailed
	}
	var exitErr *exec.ExitError
	retryable := !errors.As(err, &exitErr)
	return &AdapterError{Code: code, Message: limitOutput(message), Retryable: retryable}
}

func limitOutput(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 2000 {
		return value
	}
	return value[:1997] + "..."
}
