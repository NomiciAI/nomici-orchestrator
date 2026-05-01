package trace

import (
	"encoding/json"
	"time"
)

type Event struct {
	EventID    string          `json:"event_id"`
	RunID      string          `json:"run_id"`
	Sequence   int             `json:"sequence"`
	Type       string          `json:"type"`
	Time       time.Time       `json:"time"`
	NodeID     string          `json:"node_id,omitempty"`
	RuntimeID  string          `json:"runtime_id,omitempty"`
	Payload    json.RawMessage `json:"payload"`
	Redactions []string        `json:"redactions"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
}

type RunSummary struct {
	RunID      string `json:"run_id"`
	EventCount int    `json:"event_count"`
	FirstTime  string `json:"first_time"`
	LastTime   string `json:"last_time"`
	LastType   string `json:"last_type"`
}

const (
	EventRunStarted     = "run.started"
	EventRunCompleted   = "run.completed"
	EventRunFailed      = "run.failed"
	EventModelRequested = "model.requested"
	EventModelCompleted = "model.completed"
	EventModelFailed    = "model.failed"
)
