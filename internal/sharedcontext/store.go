package sharedcontext

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (store *Store) SaveItem(ctx context.Context, item *Item) error {
	if err := PrepareItem(item); err != nil {
		return err
	}
	if item.ContextID == "" {
		item.ContextID = ids.New("ctx")
	}
	now := time.Now().UTC()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = item.CreatedAt
	}

	_, err := store.db.ExecContext(ctx, `
INSERT INTO context_items (
	context_id, project_id, run_id, task_id, agent_id, agent_pair, task_type,
	scope, kind, title, body, tags_json, subject_refs_json, artifact_refs_json,
	source_json, confidence, sensitivity, status, expires_at, supersedes,
	metadata_json, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ContextID,
		item.ProjectID,
		item.RunID,
		item.TaskID,
		item.AgentID,
		item.AgentPair,
		item.TaskType,
		item.Scope,
		item.Kind,
		item.Title,
		item.Body,
		mustJSON(item.Tags),
		mustJSON(item.SubjectRefs),
		mustJSON(item.ArtifactRefs),
		mustJSON(item.Source),
		item.Confidence,
		item.Sensitivity,
		item.Status,
		item.ExpiresAt,
		item.Supersedes,
		mustJSON(item.Metadata),
		item.CreatedAt.Format(time.RFC3339Nano),
		item.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("save context item: %w", err)
	}
	return nil
}

func (store *Store) SaveSnapshot(ctx context.Context, snapshot *Snapshot) error {
	if err := PrepareSnapshot(snapshot); err != nil {
		return err
	}
	if snapshot.SnapshotID == "" {
		snapshot.SnapshotID = ids.New("ctxsnap")
	}
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = time.Now().UTC()
	}

	_, err := store.db.ExecContext(ctx, `
INSERT INTO context_snapshots (
	snapshot_id, project_id, run_id, task_id, from_agent, to_agent,
	summary, decisions_json, open_issues_json, recommendations_json,
	artifact_refs_json, context_item_refs_json, created_by_json,
	supersedes, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snapshot.SnapshotID,
		snapshot.ProjectID,
		snapshot.RunID,
		snapshot.TaskID,
		snapshot.FromAgent,
		snapshot.ToAgent,
		snapshot.Summary,
		mustJSON(snapshot.Decisions),
		mustJSON(snapshot.OpenIssues),
		mustJSON(snapshot.Recommendations),
		mustJSON(snapshot.ArtifactRefs),
		mustJSON(snapshot.ContextItemRefs),
		mustJSON(snapshot.CreatedBy),
		snapshot.Supersedes,
		snapshot.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("save context snapshot: %w", err)
	}
	return nil
}

func (store *Store) ListSnapshots(ctx context.Context, projectID string, limit int) ([]*Snapshot, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
SELECT snapshot_id, project_id, run_id, task_id, from_agent, to_agent,
	summary, decisions_json, open_issues_json, recommendations_json,
	artifact_refs_json, context_item_refs_json, created_by_json,
	supersedes, created_at
FROM context_snapshots`
	args := []any{}
	if projectID != "" {
		query += " WHERE project_id = ?"
		args = append(args, projectID)
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list context snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []*Snapshot
	for rows.Next() {
		snapshot, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list context snapshots: %w", err)
	}
	return snapshots, nil
}

type snapshotScanner interface {
	Scan(dest ...any) error
}

func scanSnapshot(row snapshotScanner) (*Snapshot, error) {
	var snapshot Snapshot
	var decisionsJSON string
	var openIssuesJSON string
	var recommendationsJSON string
	var artifactRefsJSON string
	var contextItemRefsJSON string
	var createdByJSON string
	var createdAt string
	if err := row.Scan(
		&snapshot.SnapshotID,
		&snapshot.ProjectID,
		&snapshot.RunID,
		&snapshot.TaskID,
		&snapshot.FromAgent,
		&snapshot.ToAgent,
		&snapshot.Summary,
		&decisionsJSON,
		&openIssuesJSON,
		&recommendationsJSON,
		&artifactRefsJSON,
		&contextItemRefsJSON,
		&createdByJSON,
		&snapshot.Supersedes,
		&createdAt,
	); err != nil {
		return nil, fmt.Errorf("scan context snapshot: %w", err)
	}
	if err := json.Unmarshal([]byte(decisionsJSON), &snapshot.Decisions); err != nil {
		return nil, fmt.Errorf("decode context decisions: %w", err)
	}
	if err := json.Unmarshal([]byte(openIssuesJSON), &snapshot.OpenIssues); err != nil {
		return nil, fmt.Errorf("decode context open issues: %w", err)
	}
	if err := json.Unmarshal([]byte(recommendationsJSON), &snapshot.Recommendations); err != nil {
		return nil, fmt.Errorf("decode context recommendations: %w", err)
	}
	if err := json.Unmarshal([]byte(artifactRefsJSON), &snapshot.ArtifactRefs); err != nil {
		return nil, fmt.Errorf("decode context artifacts: %w", err)
	}
	if err := json.Unmarshal([]byte(contextItemRefsJSON), &snapshot.ContextItemRefs); err != nil {
		return nil, fmt.Errorf("decode context item refs: %w", err)
	}
	if err := json.Unmarshal([]byte(createdByJSON), &snapshot.CreatedBy); err != nil {
		return nil, fmt.Errorf("decode context creator: %w", err)
	}
	parsedTime, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse context snapshot time: %w", err)
	}
	snapshot.CreatedAt = parsedTime
	return &snapshot, nil
}

func mustJSON(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(payload)
}
