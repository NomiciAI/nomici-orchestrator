package policy

import "time"

const (
	ActionCLIInvoke = "runtime.cli_agent.invoke"

	DecisionAllow    = "allow"
	DecisionDeny     = "deny"
	DecisionApproval = "approval"

	RiskLow      = "low"
	RiskMedium   = "medium"
	RiskHigh     = "high"
	RiskCritical = "critical"

	StatusPending = "pending"
	StatusGranted = "granted"
	StatusDenied  = "denied"

	ScopeOnce = "once"
	ScopeRun  = "run"
)

type ActionRequest struct {
	RunID      string            `json:"run_id"`
	ActionID   string            `json:"action_id"`
	ActionType string            `json:"action_type"`
	ProjectID  string            `json:"project_id"`
	AgentID    string            `json:"agent_id"`
	RuntimeID  string            `json:"runtime_id"`
	Workspace  string            `json:"workspace"`
	FilesWrite bool              `json:"files_write"`
	Summary    string            `json:"summary"`
	Subject    map[string]string `json:"subject,omitempty"`
}

type Decision struct {
	Decision    string    `json:"decision"`
	Risk        string    `json:"risk"`
	Reason      string    `json:"reason"`
	Fingerprint string    `json:"fingerprint"`
	ApprovalID  string    `json:"approval_id,omitempty"`
	Scope       string    `json:"scope,omitempty"`
	RequestedAt time.Time `json:"requested_at,omitempty"`
}

type Approval struct {
	ApprovalID        string            `json:"approval_id"`
	RunID             string            `json:"run_id"`
	ActionID          string            `json:"action_id"`
	ActionType        string            `json:"action_type"`
	ActionFingerprint string            `json:"action_fingerprint"`
	Status            string            `json:"status"`
	Risk              string            `json:"risk"`
	Scope             string            `json:"scope,omitempty"`
	Summary           string            `json:"summary"`
	Subject           map[string]string `json:"subject,omitempty"`
	RequestedByAgent  string            `json:"requested_by_agent,omitempty"`
	RuntimeID         string            `json:"runtime_id,omitempty"`
	Reason            string            `json:"reason,omitempty"`
	RequestedAt       time.Time         `json:"requested_at"`
	ResolvedAt        string            `json:"resolved_at,omitempty"`
	ConsumedAt        string            `json:"consumed_at,omitempty"`
	BoundRunID        string            `json:"bound_run_id,omitempty"`
}
