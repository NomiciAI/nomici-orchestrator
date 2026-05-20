package sharedcontext

import "time"

const (
	ScopeProject = "project"
	ScopeRun     = "run"
	ScopeHandoff = "handoff"

	KindRunSummary      = "run_summary"
	KindHandoffBriefing = "handoff_briefing"

	StatusActive  = "active"
	StatusDeleted = "deleted"

	ConfidenceGenerated = "generated"

	SensitivityNormal = "normal"
)

type Item struct {
	ContextID    string            `json:"context_id"`
	ProjectID    string            `json:"project_id"`
	RunID        string            `json:"run_id,omitempty"`
	TaskID       string            `json:"task_id,omitempty"`
	AgentID      string            `json:"agent_id,omitempty"`
	AgentPair    string            `json:"agent_pair,omitempty"`
	TaskType     string            `json:"task_type,omitempty"`
	Scope        string            `json:"scope"`
	Kind         string            `json:"kind"`
	Title        string            `json:"title"`
	Body         string            `json:"body"`
	Tags         []string          `json:"tags,omitempty"`
	SubjectRefs  []SubjectRef      `json:"subject_refs,omitempty"`
	ArtifactRefs []string          `json:"artifact_refs,omitempty"`
	Source       Source            `json:"source"`
	Confidence   string            `json:"confidence"`
	Sensitivity  string            `json:"sensitivity"`
	Status       string            `json:"status"`
	ExpiresAt    string            `json:"expires_at,omitempty"`
	Supersedes   string            `json:"supersedes,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

type SubjectRef struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type Source struct {
	Kind    string `json:"kind"`
	RunID   string `json:"run_id,omitempty"`
	TaskID  string `json:"task_id,omitempty"`
	AgentID string `json:"agent_id,omitempty"`
	EventID string `json:"event_id,omitempty"`
}

type Snapshot struct {
	SnapshotID      string    `json:"snapshot_id"`
	ProjectID       string    `json:"project_id"`
	RunID           string    `json:"run_id"`
	TaskID          string    `json:"task_id,omitempty"`
	FromAgent       string    `json:"from_agent,omitempty"`
	ToAgent         string    `json:"to_agent,omitempty"`
	Summary         string    `json:"summary"`
	Decisions       []Note    `json:"decisions,omitempty"`
	OpenIssues      []string  `json:"open_issues,omitempty"`
	Recommendations []string  `json:"recommendations,omitempty"`
	ArtifactRefs    []string  `json:"artifact_refs,omitempty"`
	ContextItemRefs []string  `json:"context_item_refs,omitempty"`
	CreatedBy       CreatedBy `json:"created_by"`
	Supersedes      string    `json:"supersedes,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type Note struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type CreatedBy struct {
	Kind    string `json:"kind"`
	AgentID string `json:"agent_id,omitempty"`
}

type Briefing struct {
	SnapshotID string `json:"snapshot_id,omitempty"`
	Text       string `json:"text,omitempty"`
}
