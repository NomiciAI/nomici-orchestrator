package blocked

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
)

const (
	KindPlanReview    = "plan_review"
	KindToolApproval  = "tool_approval"
	KindClarification = "clarification"

	StatusOpen     = "open"
	StatusResolved = "resolved"
	StatusRejected = "rejected"
	StatusDeleted  = "deleted"
)

type Action struct {
	BlockedActionID    string          `json:"blocked_action_id"`
	SessionID          string          `json:"session_id"`
	RunID              string          `json:"run_id"`
	TaskID             string          `json:"task_id,omitempty"`
	Kind               string          `json:"kind"`
	Status             string          `json:"status"`
	Title              string          `json:"title"`
	Body               string          `json:"body,omitempty"`
	RequiredAction     string          `json:"required_action,omitempty"`
	ResumeTargetTaskID string          `json:"resume_target_task_id,omitempty"`
	ApprovalID         string          `json:"approval_id,omitempty"`
	ArtifactID         string          `json:"artifact_id,omitempty"`
	ToolCallID         string          `json:"tool_call_id,omitempty"`
	Metadata           json.RawMessage `json:"metadata,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	ResolvedAt         string          `json:"resolved_at,omitempty"`
}

type CreateRequest struct {
	SessionID          string
	RunID              string
	TaskID             string
	Kind               string
	Title              string
	Body               string
	RequiredAction     string
	ResumeTargetTaskID string
	ApprovalID         string
	ArtifactID         string
	ToolCallID         string
	Metadata           json.RawMessage
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (store *Store) Create(ctx context.Context, request CreateRequest) (*Action, error) {
	if request.SessionID == "" || request.RunID == "" {
		return nil, fmt.Errorf("create blocked action: session_id and run_id are required")
	}
	if request.Kind == "" {
		return nil, fmt.Errorf("create blocked action: kind is required")
	}
	if request.Title == "" {
		request.Title = request.Kind
	}
	if len(request.Metadata) == 0 {
		request.Metadata = json.RawMessage("{}")
	}
	now := time.Now().UTC()
	action := &Action{
		BlockedActionID:    ids.New("block"),
		SessionID:          request.SessionID,
		RunID:              request.RunID,
		TaskID:             request.TaskID,
		Kind:               request.Kind,
		Status:             StatusOpen,
		Title:              request.Title,
		Body:               request.Body,
		RequiredAction:     request.RequiredAction,
		ResumeTargetTaskID: request.ResumeTargetTaskID,
		ApprovalID:         request.ApprovalID,
		ArtifactID:         request.ArtifactID,
		ToolCallID:         request.ToolCallID,
		Metadata:           request.Metadata,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	_, err := store.db.ExecContext(ctx, `
INSERT INTO blocked_actions (
	blocked_action_id, session_id, run_id, task_id, kind, status, title, body,
	required_action, resume_target_task_id, approval_id, artifact_id, tool_call_id,
	metadata_json, created_at, updated_at, resolved_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '')`,
		action.BlockedActionID,
		action.SessionID,
		action.RunID,
		action.TaskID,
		action.Kind,
		action.Status,
		action.Title,
		action.Body,
		action.RequiredAction,
		action.ResumeTargetTaskID,
		action.ApprovalID,
		action.ArtifactID,
		action.ToolCallID,
		string(action.Metadata),
		action.CreatedAt.Format(time.RFC3339Nano),
		action.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("create blocked action: %w", err)
	}
	return action, nil
}

func (store *Store) ListBySession(ctx context.Context, sessionID string, status string, limit int) ([]*Action, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("list blocked actions: session_id is required")
	}
	if limit <= 0 {
		limit = 50
	}
	query := `
SELECT blocked_action_id, session_id, run_id, task_id, kind, status, title, body,
	required_action, resume_target_task_id, approval_id, artifact_id, tool_call_id,
	metadata_json, created_at, updated_at, resolved_at
FROM blocked_actions
WHERE session_id = ?`
	args := []any{sessionID}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY updated_at DESC LIMIT ?"
	args = append(args, limit)
	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list blocked actions: %w", err)
	}
	defer rows.Close()
	var actions []*Action
	for rows.Next() {
		action, err := scanAction(rows)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list blocked actions: %w", err)
	}
	return actions, nil
}

func (store *Store) Get(ctx context.Context, actionID string) (*Action, error) {
	row := store.db.QueryRowContext(ctx, `
SELECT blocked_action_id, session_id, run_id, task_id, kind, status, title, body,
	required_action, resume_target_task_id, approval_id, artifact_id, tool_call_id,
	metadata_json, created_at, updated_at, resolved_at
FROM blocked_actions
WHERE blocked_action_id = ?`, actionID)
	return scanAction(row)
}

func (store *Store) Resolve(ctx context.Context, actionID string, metadata json.RawMessage) (*Action, error) {
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}
	return store.setStatus(ctx, "blocked_action_id = ?", actionID, StatusResolved, metadata)
}

func (store *Store) ResolveByApproval(ctx context.Context, approvalID string, status string) error {
	if approvalID == "" {
		return nil
	}
	if status == "" {
		status = StatusResolved
	}
	_, err := store.setStatus(ctx, "approval_id = ?", approvalID, status, nil)
	return err
}

func (store *Store) ResolveByArtifact(ctx context.Context, artifactID string) error {
	if artifactID == "" {
		return nil
	}
	_, err := store.setStatus(ctx, "artifact_id = ?", artifactID, StatusResolved, nil)
	return err
}

func (store *Store) setStatus(ctx context.Context, predicate string, value string, status string, metadata json.RawMessage) (*Action, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	metadataSet := ""
	args := []any{status, now, now}
	if len(metadata) > 0 {
		metadataSet = ", metadata_json = ?"
		args = append(args, string(metadata))
	}
	args = append(args, value)
	_, err := store.db.ExecContext(ctx, `
UPDATE blocked_actions
SET status = ?, updated_at = ?, resolved_at = ?`+metadataSet+`
WHERE `+predicate+` AND status = ?`, append(args, StatusOpen)...)
	if err != nil {
		return nil, fmt.Errorf("update blocked action: %w", err)
	}
	if predicate == "blocked_action_id = ?" {
		return store.Get(ctx, value)
	}
	return nil, nil
}

type actionScanner interface {
	Scan(dest ...any) error
}

func scanAction(scanner actionScanner) (*Action, error) {
	var action Action
	var metadataJSON string
	var createdAt string
	var updatedAt string
	if err := scanner.Scan(
		&action.BlockedActionID,
		&action.SessionID,
		&action.RunID,
		&action.TaskID,
		&action.Kind,
		&action.Status,
		&action.Title,
		&action.Body,
		&action.RequiredAction,
		&action.ResumeTargetTaskID,
		&action.ApprovalID,
		&action.ArtifactID,
		&action.ToolCallID,
		&metadataJSON,
		&createdAt,
		&updatedAt,
		&action.ResolvedAt,
	); err != nil {
		return nil, fmt.Errorf("scan blocked action: %w", err)
	}
	var err error
	action.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse blocked action created_at: %w", err)
	}
	action.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse blocked action updated_at: %w", err)
	}
	action.Metadata = json.RawMessage(metadataJSON)
	return &action, nil
}
