package artifacts

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
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

type Revision struct {
	RevisionID  string          `json:"revision_id"`
	ArtifactID  string          `json:"artifact_id"`
	SessionID   string          `json:"session_id"`
	RunID       string          `json:"run_id"`
	TaskID      string          `json:"task_id,omitempty"`
	Revision    int             `json:"revision"`
	ReviewState string          `json:"review_state"`
	Path        string          `json:"path,omitempty"`
	Preview     string          `json:"preview,omitempty"`
	DiffPreview string          `json:"diff_preview,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
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
	if err := store.recordRevision(ctx, artifact, ""); err != nil {
		return nil, err
	}
	return artifact, nil
}

func (store *Store) Revise(ctx context.Context, artifactID string, preview string) (*Artifact, error) {
	if artifactID == "" {
		return nil, fmt.Errorf("revise artifact: artifact_id is required")
	}
	before, err := store.Get(ctx, artifactID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = store.db.ExecContext(ctx, `
UPDATE artifact_records
SET preview = ?, revision = revision + 1, review_state = ?, updated_at = ?
WHERE artifact_id = ?`, preview, ReviewRevised, now, artifactID)
	if err != nil {
		return nil, fmt.Errorf("revise artifact: %w", err)
	}
	after, err := store.Get(ctx, artifactID)
	if err != nil {
		return nil, err
	}
	if err := store.recordRevision(ctx, after, diffPreview(before.Preview, after.Preview)); err != nil {
		return nil, err
	}
	return after, nil
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

func (store *Store) ListRevisions(ctx context.Context, artifactID string, limit int) ([]*Revision, error) {
	if artifactID == "" {
		return nil, fmt.Errorf("list artifact revisions: artifact_id is required")
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := store.db.QueryContext(ctx, `
SELECT revision_id, artifact_id, session_id, run_id, task_id, revision,
	review_state, path, preview, diff_preview, metadata_json, created_at
FROM artifact_revision_records
WHERE artifact_id = ?
ORDER BY revision DESC
LIMIT ?`, artifactID, limit)
	if err != nil {
		return nil, fmt.Errorf("list artifact revisions: %w", err)
	}
	defer rows.Close()
	var revisions []*Revision
	for rows.Next() {
		revision, err := scanRevision(rows)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, revision)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list artifact revisions: %w", err)
	}
	return revisions, nil
}

func (store *Store) recordRevision(ctx context.Context, artifact *Artifact, diff string) error {
	if artifact == nil {
		return nil
	}
	if len(artifact.Metadata) == 0 {
		artifact.Metadata = json.RawMessage("{}")
	}
	_, err := store.db.ExecContext(ctx, `
INSERT INTO artifact_revision_records (
	revision_id, artifact_id, session_id, run_id, task_id, revision, review_state,
	path, preview, diff_preview, metadata_json, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ids.New("artifactrev"),
		artifact.ArtifactID,
		artifact.SessionID,
		artifact.RunID,
		artifact.TaskID,
		artifact.Revision,
		artifact.ReviewState,
		artifact.Path,
		artifact.Preview,
		diff,
		string(artifact.Metadata),
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("record artifact revision: %w", err)
	}
	return nil
}

type artifactScanner interface {
	Scan(dest ...any) error
}

type revisionScanner interface {
	Scan(dest ...any) error
}

func scanRevision(scanner revisionScanner) (*Revision, error) {
	var revision Revision
	var metadataJSON string
	var createdAt string
	if err := scanner.Scan(
		&revision.RevisionID,
		&revision.ArtifactID,
		&revision.SessionID,
		&revision.RunID,
		&revision.TaskID,
		&revision.Revision,
		&revision.ReviewState,
		&revision.Path,
		&revision.Preview,
		&revision.DiffPreview,
		&metadataJSON,
		&createdAt,
	); err != nil {
		return nil, fmt.Errorf("scan artifact revision: %w", err)
	}
	var err error
	revision.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse artifact revision created_at: %w", err)
	}
	revision.Metadata = json.RawMessage(metadataJSON)
	return &revision, nil
}

func diffPreview(before string, after string) string {
	before = strings.TrimSpace(before)
	after = strings.TrimSpace(after)
	if before == after {
		return "No preview changes."
	}
	if len(before) > 800 {
		before = before[:800] + "..."
	}
	if len(after) > 800 {
		after = after[:800] + "..."
	}
	return "--- previous\n" + before + "\n+++ current\n" + after
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
