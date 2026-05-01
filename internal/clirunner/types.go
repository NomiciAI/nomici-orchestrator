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
	RunID         string
	TaskID        string
	Prompt        string
	Briefing      string
	SharedContext SharedContext
}

type SharedContext struct {
	SnapshotID string
	Briefing   string
}

type Result struct {
	Status          string
	ExitCode        int
	Workspace       string
	ArtifactDir     string
	StdoutRef       string
	StderrRef       string
	PreDiffRef      string
	DiffRef         string
	Stdout          string
	Stderr          string
	ChangedFiles    []string
	Error           string
	ContextSnapshot *ContextSnapshotCandidate
	StartedAt       time.Time
	CompletedAt     time.Time
}

type ContextSnapshotCandidate struct {
	Summary         string        `json:"summary"`
	Decisions       []ContextNote `json:"decisions,omitempty"`
	OpenIssues      []string      `json:"open_issues,omitempty"`
	Recommendations []string      `json:"recommendations,omitempty"`
	ArtifactRefs    []string      `json:"artifact_refs,omitempty"`
}

type ContextNote struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}
