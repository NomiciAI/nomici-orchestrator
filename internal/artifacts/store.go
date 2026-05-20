package artifacts

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
)

const (
	TypePlan   = "plan"
	TypeReport = "report"
	TypeFile   = "file"
	TypeDiff   = "diff"

	ReviewDraft    = "draft"
	ReviewApproved = "approved"
	ReviewRevised  = "revised"
)

type Store struct {
	db *sql.DB
}

type Artifact struct {
	ArtifactID  string          `json:"artifact_id"`
	SessionID   string          `json:"session_id"`
	RunID       string          `json:"run_id"`
	TaskID      string          `json:"task_id,omitempty"`
	Type        string          `json:"type"`
	Title       string          `json:"title"`
	Path        string          `json:"path,omitempty"`
	Revision    int             `json:"revision"`
	ReviewState string          `json:"review_state"`
	Preview     string          `json:"preview,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type CreateRequest struct {
	SessionID   string
	RunID       string
	TaskID      string
	Type        string
	Title       string
	Path        string
	Revision    int
	ReviewState string
	Preview     string
	Metadata    json.RawMessage
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (store *Store) Create(ctx context.Context, request CreateRequest) (*Artifact, error) {
	if request.SessionID == "" {
		return nil, fmt.Errorf("create artifact: session_id is required")
	}
	if request.RunID == "" {
		return nil, fmt.Errorf("create artifact: run_id is required")
	}
	if request.Type == "" {
		return nil, fmt.Errorf("create artifact: type is required")
	}
	if request.Title == "" {
		request.Title = request.Type
	}
	if request.Revision == 0 {
		request.Revision = 1
	}
	if request.ReviewState == "" {
		request.ReviewState = ReviewDraft
	}
	if len(request.Metadata) == 0 {
		request.Metadata = json.RawMessage("{}")
	}
	now := time.Now().UTC()
	artifact := &Artifact{
		ArtifactID:  ids.New("artifact"),
		SessionID:   request.SessionID,
		RunID:       request.RunID,
		TaskID:      request.TaskID,
		Type:        request.Type,
		Title:       request.Title,
		Path:        request.Path,
		Revision:    request.Revision,
		ReviewState: request.ReviewState,
		Preview:     request.Preview,
		Metadata:    request.Metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err := store.db.ExecContext(ctx, `
INSERT INTO artifact_records (
	artifact_id, session_id, run_id, task_id, type, title, path, revision,
	review_state, preview, metadata_json, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		artifact.ArtifactID,
		artifact.SessionID,
		artifact.RunID,
		artifact.TaskID,
		artifact.Type,
		artifact.Title,
		artifact.Path,
		artifact.Revision,
		artifact.ReviewState,
		artifact.Preview,
		string(artifact.Metadata),
		artifact.CreatedAt.Format(time.RFC3339Nano),
		artifact.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("create artifact: %w", err)
	}
	return artifact, nil
}

func (store *Store) Revise(ctx context.Context, artifactID string, preview string) (*Artifact, error) {
	if artifactID == "" {
		return nil, fmt.Errorf("revise artifact: artifact_id is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := store.db.ExecContext(ctx, `
UPDATE artifact_records
SET preview = ?, revision = revision + 1, review_state = ?, updated_at = ?
WHERE artifact_id = ?`, preview, ReviewRevised, now, artifactID)
	if err != nil {
		return nil, fmt.Errorf("revise artifact: %w", err)
	}
	return store.Get(ctx, artifactID)
}

func (store *Store) SetReviewState(ctx context.Context, artifactID string, state string) (*Artifact, error) {
	if artifactID == "" {
		return nil, fmt.Errorf("set artifact review state: artifact_id is required")
	}
	if state == "" {
		state = ReviewApproved
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := store.db.ExecContext(ctx, `
UPDATE artifact_records
SET review_state = ?, updated_at = ?
WHERE artifact_id = ?`, state, now, artifactID)
	if err != nil {
		return nil, fmt.Errorf("set artifact review state: %w", err)
	}
	return store.Get(ctx, artifactID)
}

func (store *Store) List(ctx context.Context, sessionID string, limit int) ([]*Artifact, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows *sql.Rows
	var err error
	if sessionID == "" {
		rows, err = store.db.QueryContext(ctx, `
SELECT artifact_id, session_id, run_id, task_id, type, title, path, revision,
	review_state, preview, metadata_json, created_at, updated_at
FROM artifact_records
ORDER BY updated_at DESC
LIMIT ?`, limit)
	} else {
		rows, err = store.db.QueryContext(ctx, `
SELECT artifact_id, session_id, run_id, task_id, type, title, path, revision,
	review_state, preview, metadata_json, created_at, updated_at
FROM artifact_records
WHERE session_id = ?
ORDER BY updated_at DESC
LIMIT ?`, sessionID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list artifacts: %w", err)
	}
	defer rows.Close()
	var artifacts []*Artifact
	for rows.Next() {
		artifact, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list artifacts: %w", err)
	}
	return artifacts, nil
}

func (store *Store) Get(ctx context.Context, artifactID string) (*Artifact, error) {
	row := store.db.QueryRowContext(ctx, `
SELECT artifact_id, session_id, run_id, task_id, type, title, path, revision,
	review_state, preview, metadata_json, created_at, updated_at
FROM artifact_records
WHERE artifact_id = ?`, artifactID)
	return scanArtifact(row)
}

type artifactScanner interface {
	Scan(dest ...any) error
}

func scanArtifact(scanner artifactScanner) (*Artifact, error) {
	var artifact Artifact
	var metadataJSON string
	var createdAt string
	var updatedAt string
	if err := scanner.Scan(
		&artifact.ArtifactID,
		&artifact.SessionID,
		&artifact.RunID,
		&artifact.TaskID,
		&artifact.Type,
		&artifact.Title,
		&artifact.Path,
		&artifact.Revision,
		&artifact.ReviewState,
		&artifact.Preview,
		&metadataJSON,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan artifact: %w", err)
	}
	var err error
	artifact.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse artifact created_at: %w", err)
	}
	artifact.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse artifact updated_at: %w", err)
	}
	artifact.Metadata = json.RawMessage(metadataJSON)
	return &artifact, nil
}
