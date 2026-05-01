package lifecycle

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
)

func TestStartStopLocalProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process signal semantics differ on Windows")
	}
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep command not available")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state.db")
	state, err := StartLocalProcess(context.Background(), StartRuntimeOptions{
		RuntimeID: "worker",
		Runtime: agentspec.Runtime{
			Kind: agentspec.RuntimeKindLocalProcess,
			Start: agentspec.RuntimeStart{
				Executable: sleep,
				Args:       []string{"30"},
			},
		},
		ConfigPath: filepath.Join(dir, "nomici.yaml"),
		DBPath:     dbPath,
	})
	if err != nil {
		t.Fatalf("start local process: %v", err)
	}
	if state.PID == 0 {
		t.Fatal("expected runtime pid")
	}
	if !ProcessAlive(state.PID) {
		t.Fatal("expected started process to be alive")
	}
	if state.LogPath == "" {
		t.Fatal("expected log path")
	}

	stopped, err := Stop(NewPaths(dbPath), "worker")
	if err != nil {
		t.Fatalf("stop local process: %v", err)
	}
	if stopped.Status != StatusStopped {
		t.Fatalf("expected stopped status, got %q", stopped.Status)
	}
	if stopped.PID != 0 {
		t.Fatalf("expected pid to be cleared, got %d", stopped.PID)
	}
}
