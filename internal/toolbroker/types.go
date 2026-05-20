package toolbroker

import (
	"encoding/json"
	"time"
)

const (
	StatusPending         = "pending"
	StatusWaitingApproval = "waiting_approval"
	StatusRunning         = "running"
	StatusCompleted       = "completed"
	StatusFailed          = "failed"
	StatusDenied          = "denied"

	RiskLow      = "low"
	RiskMedium   = "medium"
	RiskHigh     = "high"
	RiskCritical = "critical"

	ToolListFiles       = "list_files"
	ToolReadFile        = "read_file"
	ToolWriteFile       = "write_file"
	ToolReplaceFile     = "replace_file"
	ToolPresentArtifact = "present_artifact"
	ToolBash            = "bash"
	ToolSearch          = "search"
	ToolFetch           = "fetch"
)

type Definition struct {
	ID             string   `json:"id"`
	Description    string   `json:"description"`
	Auth           string   `json:"auth"`
	NetworkRisk    string   `json:"network_risk"`
	FilesystemRisk string   `json:"filesystem_risk"`
	MutationRisk   string   `json:"mutation_risk"`
	AllowedScopes  []string `json:"allowed_scopes"`
	RedactionRules []string `json:"redaction_rules"`
	Execution      string   `json:"execution"`
}

type CallRecord struct {
	ToolCallID    string          `json:"tool_call_id"`
	SessionID     string          `json:"session_id"`
	RunID         string          `json:"run_id"`
	TaskID        string          `json:"task_id,omitempty"`
	ToolID        string          `json:"tool_id"`
	Status        string          `json:"status"`
	Risk          string          `json:"risk,omitempty"`
	InputPreview  string          `json:"input_preview,omitempty"`
	OutputPreview string          `json:"output_preview,omitempty"`
	ArtifactRefs  []string        `json:"artifact_refs"`
	ApprovalID    string          `json:"approval_id,omitempty"`
	Error         string          `json:"error,omitempty"`
	Redactions    []string        `json:"redactions,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	CompletedAt   string          `json:"completed_at,omitempty"`
}

type CreateCallRequest struct {
	SessionID    string
	RunID        string
	TaskID       string
	ToolID       string
	Status       string
	Risk         string
	InputPreview string
	Metadata     json.RawMessage
}

type ExecuteRequest struct {
	SessionID string         `json:"session_id,omitempty"`
	RunID     string         `json:"run_id,omitempty"`
	TaskID    string         `json:"task_id,omitempty"`
	AgentID   string         `json:"agent_id,omitempty"`
	ToolID    string         `json:"tool_id"`
	Input     map[string]any `json:"input,omitempty"`
}
