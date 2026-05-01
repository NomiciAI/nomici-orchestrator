package clirunner

import "time"

const (
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

type Config struct {
	RuntimeID      string
	AgentID        string
	Workspace      string
	Executable     string
	Args           []string
	Stdin          string
	Env            map[string]string
	EnvFrom        []string
	TimeoutSeconds int
	FilesWrite     bool
}

type Request struct {
	RunID    string
	TaskID   string
	Prompt   string
	Briefing string
}

type Result struct {
	Status       string
	ExitCode     int
	Workspace    string
	ArtifactDir  string
	StdoutRef    string
	StderrRef    string
	PreDiffRef   string
	DiffRef      string
	Stdout       string
	Stderr       string
	ChangedFiles []string
	Error        string
	StartedAt    time.Time
	CompletedAt  time.Time
}
