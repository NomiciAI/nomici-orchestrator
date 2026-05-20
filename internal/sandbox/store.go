package sandbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (store *Store) CreateForRun(ctx context.Context, request CreateRecordRequest) (*Record, error) {
	if request.RunID == "" {
		return nil, fmt.Errorf("create sandbox record: run_id is required")
	}
	intent := request.Intent
	intent.Mode = NormalizeMode(intent.Mode)
	availability := Detect(intent.Mode)
	workspaceRoot := request.WorkspaceRoot
	artifactRoot := request.ArtifactRoot
	cleanupStatus := CleanupActive
	if intent.Mode == ModeNone {
		cleanupStatus = CleanupDisabled
	} else {
		if workspaceRoot == "" {
			workspaceRoot = DefaultWorkspaceRoot(request.BaseDir, request.RunID)
		}
		if artifactRoot == "" {
			artifactRoot = DefaultArtifactRoot(request.BaseDir, request.RunID)
		}
		if err := os.MkdirAll(workspaceRoot, 0o700); err != nil {
			return nil, fmt.Errorf("create sandbox workspace: %w", err)
		}
		if err := os.MkdirAll(artifactRoot, 0o700); err != nil {
			return nil, fmt.Errorf("create sandbox artifact root: %w", err)
		}
	}
	metadata := request.Metadata
	if len(metadata) == 0 {
		var err error
		metadata, err = json.Marshal(map[string]any{
			"project_id":          request.ProjectID,
			"bash_enabled":        intent.BashEnabled,
			"file_write_enabled":  intent.FileWriteEnabled,
			"workspace_mount":     "/workspace",
			"artifact_mount":      "/artifacts",
			"base_dir":            request.BaseDir,
			"design_note":         "SandboxProvider acquire/get/release lifecycle with per-run workspace mounts.",
			"allocation_strategy": "deterministic_run_id",
		})
		if err != nil {
			return nil, err
		}
	}

	now := time.Now().UTC()
	record := &Record{
		SandboxID:     DeterministicID(request.RunID),
		RunID:         request.RunID,
		TaskID:        request.TaskID,
		Provider:      availability.Provider,
		Mode:          availability.Mode,
		Status:        availability.Status,
		WorkspaceRoot: workspaceRoot,
		ArtifactRoot:  artifactRoot,
		RuntimeBinary: availability.RuntimeBinary,
		CleanupStatus: cleanupStatus,
		Message:       availability.Message,
		CreatedAt:     now,
		UpdatedAt:     now,
		Metadata:      metadata,
	}

	_, err := store.db.ExecContext(ctx, `
INSERT INTO sandbox_records (
	sandbox_id, run_id, task_id, provider, mode, status, workspace_root,
	artifact_root, runtime_binary, cleanup_status, message, created_at,
	updated_at, released_at, metadata_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?)`,
		record.SandboxID,
		record.RunID,
		record.TaskID,
		record.Provider,
		record.Mode,
		record.Status,
		record.WorkspaceRoot,
		record.ArtifactRoot,
		record.RuntimeBinary,
		record.CleanupStatus,
		record.Message,
		record.CreatedAt.Format(time.RFC3339Nano),
		record.UpdatedAt.Format(time.RFC3339Nano),
		string(record.Metadata),
	)
	if err != nil {
		return nil, fmt.Errorf("create sandbox record: %w", err)
	}
	return record, nil
}

func (store *Store) GetByRun(ctx context.Context, runID string) (*Record, error) {
	row := store.db.QueryRowContext(ctx, `
SELECT sandbox_id, run_id, task_id, provider, mode, status, workspace_root,
	artifact_root, runtime_binary, cleanup_status, message, created_at,
	updated_at, released_at, metadata_json
FROM sandbox_records
WHERE run_id = ?`, runID)
	return scanRecord(row)
}

func (store *Store) ReleaseByRun(ctx context.Context, runID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := store.db.ExecContext(ctx, `
UPDATE sandbox_records
SET cleanup_status = ?, updated_at = ?, released_at = ?
WHERE run_id = ? AND cleanup_status = ?`,
		CleanupReleased,
		now,
		now,
		runID,
		CleanupActive,
	)
	if err != nil {
		return fmt.Errorf("release sandbox record: %w", err)
	}
	return nil
}

type recordScanner interface {
	Scan(dest ...any) error
}

func scanRecord(scanner recordScanner) (*Record, error) {
	var record Record
	var createdAt string
	var updatedAt string
	var releasedAt string
	var metadataJSON string
	if err := scanner.Scan(
		&record.SandboxID,
		&record.RunID,
		&record.TaskID,
		&record.Provider,
		&record.Mode,
		&record.Status,
		&record.WorkspaceRoot,
		&record.ArtifactRoot,
		&record.RuntimeBinary,
		&record.CleanupStatus,
		&record.Message,
		&createdAt,
		&updatedAt,
		&releasedAt,
		&metadataJSON,
	); err != nil {
		return nil, fmt.Errorf("scan sandbox record: %w", err)
	}
	var err error
	record.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse sandbox created_at: %w", err)
	}
	record.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse sandbox updated_at: %w", err)
	}
	if releasedAt != "" {
		parsed, err := time.Parse(time.RFC3339Nano, releasedAt)
		if err != nil {
			return nil, fmt.Errorf("parse sandbox released_at: %w", err)
		}
		record.ReleasedAt = &parsed
	}
	record.Metadata = json.RawMessage(metadataJSON)
	return &record, nil
}
