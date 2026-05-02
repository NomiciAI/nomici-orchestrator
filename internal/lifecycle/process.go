package lifecycle

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/gateway"
)

type StartGatewayOptions struct {
	Executable string
	Host       string
	Port       int
	Version    string
	DBPath     string
	ConfigPath string
}

func StartGateway(ctx context.Context, options StartGatewayOptions) (State, error) {
	paths := NewPaths(options.DBPath)
	if err := paths.Ensure(); err != nil {
		return State{}, err
	}
	if state, err := LoadState(paths, RuntimeGateway); err == nil && state.PID > 0 && ProcessAlive(state.PID) {
		state.Status = StatusRunning
		return state, nil
	}
	if options.Executable == "" {
		executable, err := os.Executable()
		if err != nil {
			return State{}, fmt.Errorf("resolve current executable: %w", err)
		}
		options.Executable = executable
	}
	if options.Host == "" {
		options.Host = gateway.DefaultHost
	}
	if options.Port == 0 {
		options.Port = gateway.DefaultPort
	}
	if options.ConfigPath == "" {
		options.ConfigPath = "nomici.yaml"
	}

	args := []string{
		"gateway", "run",
		"--host", options.Host,
		"--port", fmt.Sprintf("%d", options.Port),
		"--db-path", paths.DBPath,
		"--config", options.ConfigPath,
	}
	logFile, err := openLog(paths.GatewayLog)
	if err != nil {
		return State{}, err
	}
	defer logFile.Close()

	command := exec.CommandContext(ctx, options.Executable, args...)
	command.Stdout = logFile
	command.Stderr = logFile
	detach(command)
	if err := command.Start(); err != nil {
		return State{}, fmt.Errorf("start Gateway: %w", err)
	}

	state := State{
		RuntimeID:  RuntimeGateway,
		Kind:       "gateway",
		Status:     StatusRunning,
		PID:        command.Process.Pid,
		Command:    append([]string{options.Executable}, args...),
		LogPath:    paths.GatewayLog,
		ConfigPath: options.ConfigPath,
		StartedAt:  time.Now().UTC(),
	}
	if err := SaveState(paths, state); err != nil {
		return State{}, err
	}
	if err := waitForGatewayHealth(options.Host, options.Port); err != nil {
		state.Status = StatusStartFailed
		state.LastError = err.Error()
		_ = SaveState(paths, state)
		return State{}, err
	}
	return state, nil
}

type StartRuntimeOptions struct {
	RuntimeID  string
	Runtime    agentspec.Runtime
	ConfigPath string
	DBPath     string
}

func StartLocalProcess(ctx context.Context, options StartRuntimeOptions) (State, error) {
	paths := NewPaths(options.DBPath)
	if err := paths.Ensure(); err != nil {
		return State{}, err
	}
	if state, err := LoadState(paths, options.RuntimeID); err == nil && state.PID > 0 && ProcessAlive(state.PID) {
		state.Status = StatusRunning
		return state, nil
	}

	executable, args, err := startCommand(options.Runtime.Start)
	if err != nil {
		return State{}, err
	}
	workspace := resolveWorkspace(options.Runtime.Workspace, options.ConfigPath)
	if workspace != "" {
		if err := os.MkdirAll(workspace, 0o700); err != nil {
			return State{}, fmt.Errorf("create runtime workspace: %w", err)
		}
	}
	logPath := paths.RuntimeLogPath(options.RuntimeID)
	logFile, err := openLog(logPath)
	if err != nil {
		return State{}, err
	}
	defer logFile.Close()

	command := exec.CommandContext(ctx, executable, args...)
	if workspace != "" {
		command.Dir = workspace
	}
	env, err := runtimeEnv(options.Runtime)
	if err != nil {
		return State{}, err
	}
	command.Env = env
	command.Stdout = logFile
	command.Stderr = logFile
	detach(command)
	if err := command.Start(); err != nil {
		state := State{
			RuntimeID:  options.RuntimeID,
			Kind:       options.Runtime.Kind,
			Status:     StatusStartFailed,
			Command:    append([]string{executable}, args...),
			Workspace:  workspace,
			LogPath:    logPath,
			ConfigPath: options.ConfigPath,
			LastError:  err.Error(),
		}
		_ = SaveState(paths, state)
		return State{}, fmt.Errorf("start runtime %s: %w", options.RuntimeID, err)
	}

	state := State{
		RuntimeID:  options.RuntimeID,
		Kind:       options.Runtime.Kind,
		Status:     StatusRunning,
		PID:        command.Process.Pid,
		Command:    append([]string{executable}, args...),
		Workspace:  workspace,
		LogPath:    logPath,
		ConfigPath: options.ConfigPath,
		StartedAt:  time.Now().UTC(),
	}
	if err := SaveState(paths, state); err != nil {
		return State{}, err
	}
	return state, nil
}

func Stop(paths Paths, runtimeID string) (State, error) {
	state, err := LoadState(paths, runtimeID)
	if err != nil {
		return State{}, err
	}
	if state.PID == 0 || !ProcessAlive(state.PID) {
		state.Status = StatusStopped
		state.PID = 0
		return state, SaveState(paths, state)
	}
	process, err := os.FindProcess(state.PID)
	if err != nil {
		return State{}, err
	}
	state.Status = StatusStopRequested
	_ = SaveState(paths, state)
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return State{}, fmt.Errorf("stop runtime %s: %w", runtimeID, err)
	}
	for i := 0; i < 20; i++ {
		if !ProcessAlive(state.PID) {
			state.Status = StatusStopped
			state.PID = 0
			return state, SaveState(paths, state)
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = process.Kill()
	state.Status = StatusStopped
	state.PID = 0
	return state, SaveState(paths, state)
}

func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS != "windows" {
		var status syscall.WaitStatus
		waited, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
		if err == nil && waited == pid {
			return false
		}
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return process.Signal(syscall.Signal(0)) == nil
}

func startCommand(start agentspec.RuntimeStart) (string, []string, error) {
	if strings.TrimSpace(start.Executable) != "" {
		return start.Executable, start.Args, nil
	}
	parts := strings.Fields(start.Command)
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("start command is empty")
	}
	return parts[0], parts[1:], nil
}

func resolveWorkspace(workspace string, configPath string) string {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return ""
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

func runtimeEnv(runtime agentspec.Runtime) ([]string, error) {
	allowed := map[string]bool{
		"HOME": true, "PATH": true, "SHELL": true, "TMPDIR": true, "TEMP": true, "TMP": true,
		"LANG": true, "LC_ALL": true, "USER": true, "USERNAME": true,
	}
	envMap := map[string]string{}
	for _, pair := range os.Environ() {
		key, value, ok := strings.Cut(pair, "=")
		if ok && allowed[key] {
			envMap[key] = value
		}
	}
	for key, value := range runtime.Env {
		envMap[key] = value
	}
	for _, key := range runtime.EnvFrom {
		if value, ok := os.LookupEnv(key); ok {
			envMap[key] = value
		}
	}
	env := make([]string, 0, len(envMap))
	for key, value := range envMap {
		env = append(env, key+"="+value)
	}
	return env, nil
}

func openLog(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
}

func detach(command *exec.Cmd) {
	if runtime.GOOS != "windows" {
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
}

func waitForGatewayHealth(host string, port int) error {
	url := fmt.Sprintf("http://%s:%d/api/health", host, port)
	var lastErr error
	for i := 0; i < 50; i++ {
		response, err := http.Get(url)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("health returned HTTP %d", response.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("Gateway did not become healthy at %s: %w", url, lastErr)
}
