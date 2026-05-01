package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	RuntimeGateway      = "gateway"
	StatusConfigured    = "configured"
	StatusRunning       = "running"
	StatusStopped       = "stopped"
	StatusStale         = "stale"
	StatusStartFailed   = "start_failed"
	StatusStopRequested = "stop_requested"
)

type State struct {
	RuntimeID  string    `json:"runtime_id"`
	Kind       string    `json:"kind"`
	Status     string    `json:"status"`
	PID        int       `json:"pid,omitempty"`
	Command    []string  `json:"command,omitempty"`
	Workspace  string    `json:"workspace,omitempty"`
	LogPath    string    `json:"log_path,omitempty"`
	ConfigPath string    `json:"config_path,omitempty"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
	LastError  string    `json:"last_error,omitempty"`
}

type Paths struct {
	DBPath     string
	StateDir   string
	RuntimeDir string
	LogDir     string
	GatewayLog string
}

func NewPaths(dbPath string) Paths {
	if dbPath == "" {
		dbPath = filepath.Join(".nomici", "state.db")
	}
	stateDir := filepath.Dir(dbPath)
	return Paths{
		DBPath:     dbPath,
		StateDir:   stateDir,
		RuntimeDir: filepath.Join(stateDir, "runtimes"),
		LogDir:     filepath.Join(stateDir, "logs"),
		GatewayLog: filepath.Join(stateDir, "logs", "gateway.log"),
	}
}

func (paths Paths) Ensure() error {
	for _, dir := range []string{paths.StateDir, paths.RuntimeDir, paths.LogDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create lifecycle directory %s: %w", dir, err)
		}
	}
	return nil
}

func (paths Paths) StatePath(runtimeID string) string {
	return filepath.Join(paths.RuntimeDir, runtimeID+".json")
}

func (paths Paths) RuntimeLogPath(runtimeID string) string {
	return filepath.Join(paths.LogDir, runtimeID+".log")
}

func SaveState(paths Paths, state State) error {
	if err := paths.Ensure(); err != nil {
		return err
	}
	state.UpdatedAt = time.Now().UTC()
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode runtime state: %w", err)
	}
	return os.WriteFile(paths.StatePath(state.RuntimeID), append(payload, '\n'), 0o600)
}

func LoadState(paths Paths, runtimeID string) (State, error) {
	payload, err := os.ReadFile(paths.StatePath(runtimeID))
	if err != nil {
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(payload, &state); err != nil {
		return State{}, fmt.Errorf("decode runtime state %s: %w", runtimeID, err)
	}
	return state, nil
}

func ListStates(paths Paths) ([]State, error) {
	entries, err := os.ReadDir(paths.RuntimeDir)
	if os.IsNotExist(err) {
		return []State{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list runtime states: %w", err)
	}
	var states []State
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		runtimeID := entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))]
		state, err := LoadState(paths, runtimeID)
		if err != nil {
			return nil, err
		}
		if state.PID > 0 && !ProcessAlive(state.PID) && state.Status == StatusRunning {
			state.Status = StatusStale
		}
		states = append(states, state)
	}
	sort.Slice(states, func(i, j int) bool {
		return states[i].RuntimeID < states[j].RuntimeID
	})
	return states, nil
}
