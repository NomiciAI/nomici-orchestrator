package clirunner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvokeCapturesArtifactsAndManifestDiff(t *testing.T) {
	workspace := t.TempDir()
	script := writeScript(t, `
#!/bin/sh
echo "stdout:$1"
echo "stderr:$NOMICI_TEST_TOKEN" >&2
printf "%s" "$1" > result.txt
`)
	t.Setenv("NOMICI_TEST_TOKEN", "allowed-value")

	result, err := Invoke(context.Background(), Config{
		RuntimeID:  "fake_cli",
		AgentID:    "implementer",
		Workspace:  workspace,
		Executable: script,
		Args:       []string{"${INPUT}"},
		EnvFrom:    []string{"NOMICI_TEST_TOKEN"},
		FilesWrite: true,
	}, Request{
		RunID:  "run_test",
		TaskID: "task_test",
		Prompt: "write file",
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s: %s", result.Status, result.Error)
	}
	if !strings.Contains(result.Stdout, "stdout:write file") {
		t.Fatalf("expected stdout prompt, got %q", result.Stdout)
	}
	if !strings.Contains(result.Stderr, "stderr:allowed-value") {
		t.Fatalf("expected allowed env on stderr, got %q", result.Stderr)
	}
	assertFileContains(t, result.StdoutRef, "stdout:write file")
	assertFileContains(t, result.StderrRef, "stderr:allowed-value")
	assertFileContains(t, result.DiffRef, "A result.txt")
	if len(result.ChangedFiles) != 1 || result.ChangedFiles[0] != "result.txt" {
		t.Fatalf("expected result.txt changed file, got %+v", result.ChangedFiles)
	}
}

func TestInvokeMissingExecutableReturnsFailure(t *testing.T) {
	result, err := Invoke(context.Background(), Config{
		Workspace:  t.TempDir(),
		Executable: "nomici-definitely-missing-command",
		FilesWrite: true,
	}, Request{RunID: "run_missing", Prompt: "task"})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	if !strings.Contains(result.Error, "was not found on PATH") {
		t.Fatalf("expected path error, got %q", result.Error)
	}
}

func TestInvokeRejectsMissingEnvFrom(t *testing.T) {
	result, err := Invoke(context.Background(), Config{
		Workspace:  t.TempDir(),
		Executable: writeScript(t, "#!/bin/sh\nexit 0\n"),
		EnvFrom:    []string{"NOMICI_TEST_MISSING_ENV"},
		FilesWrite: true,
	}, Request{RunID: "run_env", Prompt: "task"})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	if !strings.Contains(result.Error, "NOMICI_TEST_MISSING_ENV") {
		t.Fatalf("expected missing env error, got %q", result.Error)
	}
}

func TestInvokePreventsMutableWorkspaceConcurrency(t *testing.T) {
	workspace := t.TempDir()
	lockDir := filepath.Join(workspace, ".nomici", "locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatalf("mkdir lock dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(lockDir, lockFileName), []byte("locked"), 0o600); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	result, err := Invoke(context.Background(), Config{
		Workspace:  workspace,
		Executable: writeScript(t, "#!/bin/sh\nexit 0\n"),
		FilesWrite: true,
	}, Request{RunID: "run_lock", Prompt: "task"})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	if !strings.Contains(result.Error, "workspace is locked") {
		t.Fatalf("expected lock error, got %q", result.Error)
	}
}

func TestInvokeCapturesGitDiff(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	workspace := t.TempDir()
	runGit(t, workspace, "init")
	runGit(t, workspace, "config", "user.email", "test@example.com")
	runGit(t, workspace, "config", "user.name", "Nomici Test")
	if err := os.WriteFile(filepath.Join(workspace, "tracked.txt"), []byte("before\n"), 0o644); err != nil {
		t.Fatalf("write tracked file: %v", err)
	}
	runGit(t, workspace, "add", "tracked.txt")
	runGit(t, workspace, "commit", "-m", "initial")

	script := writeScript(t, `
#!/bin/sh
printf 'after\n' > tracked.txt
printf 'new\n' > new.txt
`)
	result, err := Invoke(context.Background(), Config{
		Workspace:  workspace,
		Executable: script,
		FilesWrite: true,
	}, Request{RunID: "run_git", Prompt: "task"})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s: %s", result.Status, result.Error)
	}
	assertFileContains(t, result.DiffRef, "diff --git")
	assertFileContains(t, result.DiffRef, "A new.txt")
	if !contains(result.ChangedFiles, "tracked.txt") || !contains(result.ChangedFiles, "new.txt") {
		t.Fatalf("expected tracked and new files, got %+v", result.ChangedFiles)
	}
}

func runGit(t *testing.T, workspace string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = workspace
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func writeScript(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-agent.sh")
	if err := os.WriteFile(path, []byte(strings.TrimLeft(content, "\n")), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return path
}

func assertFileContains(t *testing.T, path string, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(content), expected) {
		t.Fatalf("expected %s to contain %q, got %q", path, expected, string(content))
	}
}
