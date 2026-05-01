package graph

import (
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
)

type Snapshot struct {
	SnapshotID    string    `json:"snapshot_id"`
	SchemaVersion string    `json:"schema_version"`
	ProjectID     string    `json:"project_id"`
	CreatedAt     time.Time `json:"created_at"`
	SourceHash    string    `json:"source_hash"`
	IR            IR        `json:"ir"`
}

type IR struct {
	Models map[string]Model `json:"models"`
	Agents map[string]Agent `json:"agents"`
	Edges  []Edge           `json:"edges"`
}

type Model struct {
	ID            string           `json:"id"`
	Kind          string           `json:"kind"`
	BaseURL       string           `json:"base_url"`
	APIKeyEnv     string           `json:"api_key_env,omitempty"`
	Model         string           `json:"model"`
	Capabilities  []string         `json:"capabilities,omitempty"`
	ContextWindow int              `json:"context_window,omitempty"`
	Source        agentspec.Source `json:"source"`
}

type Agent struct {
	ID           string           `json:"id"`
	Kind         string           `json:"kind"`
	Model        string           `json:"model,omitempty"`
	Runtime      string           `json:"runtime,omitempty"`
	Role         string           `json:"role,omitempty"`
	Instructions string           `json:"instructions,omitempty"`
	Source       agentspec.Source `json:"source"`
}

type Edge struct {
	ID     string           `json:"id"`
	From   string           `json:"from"`
	To     string           `json:"to"`
	Mode   string           `json:"mode"`
	Source agentspec.Source `json:"source"`
}
