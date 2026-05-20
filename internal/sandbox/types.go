package sandbox

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"time"
)

const (
	ModeLocal     = "local"
	ModeContainer = "container"
	ModeNone      = "none"

	ProviderLocalWorkspace   = "local_workspace"
	ProviderContainerRuntime = "container_runtime"
	ProviderDisabled         = "disabled"

	StatusAvailable   = "available"
	StatusUnavailable = "unavailable"
	StatusDisabled    = "disabled"

	CleanupActive   = "active"
	CleanupReleased = "released"
)

type Intent struct {
	Mode             string
	BashEnabled      bool
	FileWriteEnabled bool
}

type Availability struct {
	Provider      string `json:"provider"`
	Mode          string `json:"mode"`
	Status        string `json:"status"`
	RuntimeBinary string `json:"runtime_binary,omitempty"`
	Message       string `json:"message,omitempty"`
}

type Record struct {
	SandboxID     string          `json:"sandbox_id"`
	RunID         string          `json:"run_id"`
	TaskID        string          `json:"task_id,omitempty"`
	Provider      string          `json:"provider"`
	Mode          string          `json:"mode"`
	Status        string          `json:"status"`
	WorkspaceRoot string          `json:"workspace_root,omitempty"`
	ArtifactRoot  string          `json:"artifact_root,omitempty"`
	RuntimeBinary string          `json:"runtime_binary,omitempty"`
	CleanupStatus string          `json:"cleanup_status"`
	Message       string          `json:"message,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	ReleasedAt    *time.Time      `json:"released_at,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

type CreateRecordRequest struct {
	RunID         string
	TaskID        string
	ProjectID     string
	Intent        Intent
	WorkspaceRoot string
	ArtifactRoot  string
	Metadata      json.RawMessage
}

func DefaultIntent() Intent {
	return Intent{Mode: ModeLocal, BashEnabled: false, FileWriteEnabled: false}
}

func IntentFromDeployment(deployment map[string]any) Intent {
	intent := DefaultIntent()
	raw, ok := deployment["sandbox"]
	if !ok {
		return intent
	}
	sandbox, ok := raw.(map[string]any)
	if !ok {
		return intent
	}
	if mode, ok := sandbox["mode"].(string); ok && mode != "" {
		intent.Mode = mode
	}
	if enabled, ok := sandbox["bash_enabled"].(bool); ok {
		intent.BashEnabled = enabled
	}
	if enabled, ok := sandbox["file_write_enabled"].(bool); ok {
		intent.FileWriteEnabled = enabled
	}
	return intent
}

func NormalizeMode(mode string) string {
	switch mode {
	case ModeLocal, ModeContainer, ModeNone:
		return mode
	default:
		return ModeLocal
	}
}

func DeterministicID(runID string) string {
	sum := sha256.Sum256([]byte(runID))
	return "sandbox_" + hex.EncodeToString(sum[:])[:16]
}

func DefaultWorkspaceRoot(runID string) string {
	return filepath.Join(".nomici", "runs", runID, "workspace")
}

func DefaultArtifactRoot(runID string) string {
	return filepath.Join(".nomici", "runs", runID, "artifacts")
}
