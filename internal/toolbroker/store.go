package toolbroker

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

func (store *Store) Create(ctx context.Context, request CreateCallRequest) (*CallRecord, error) {
	if request.SessionID == "" {
		return nil, fmt.Errorf("create tool call: session_id is required")
	}
	if request.RunID == "" {
		return nil, fmt.Errorf("create tool call: run_id is required")
	}
	if request.ToolID == "" {
		return nil, fmt.Errorf("create tool call: tool_id is required")
	}
	if request.Status == "" {
		request.Status = StatusPending
	}
	if len(request.Metadata) == 0 {
		request.Metadata = json.RawMessage("{}")
	}
	now := time.Now().UTC()
	record := &CallRecord{
		ToolCallID:   ids.New("toolcall"),
		SessionID:    request.SessionID,
		RunID:        request.RunID,
		TaskID:       request.TaskID,
		ToolID:       request.ToolID,
		Status:       request.Status,
		Risk:         request.Risk,
		InputPreview: request.InputPreview,
		ArtifactRefs: []string{},
		Metadata:     request.Metadata,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if _, err := store.db.ExecContext(ctx, `
INSERT INTO tool_call_records (
	tool_call_id, session_id, run_id, task_id, tool_id, status, risk,
	input_preview, output_preview, artifact_refs_json, approval_id, error,
	redactions_json, metadata_json, created_at, updated_at, completed_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ToolCallID,
		record.SessionID,
		record.RunID,
		record.TaskID,
		record.ToolID,
		record.Status,
		record.Risk,
		record.InputPreview,
		record.OutputPreview,
		mustJSON([]string{}),
		record.ApprovalID,
		record.Error,
		mustJSON([]string{}),
		string(record.Metadata),
		record.CreatedAt.Format(time.RFC3339Nano),
		record.UpdatedAt.Format(time.RFC3339Nano),
		record.CompletedAt,
	); err != nil {
		return nil, fmt.Errorf("create tool call: %w", err)
	}
	return record, nil
}

func (store *Store) MarkWaitingApproval(ctx context.Context, toolCallID string, risk string, approvalID string, reason string) (*CallRecord, error) {
	return store.update(ctx, toolCallID, StatusWaitingApproval, risk, "", approvalID, reason, nil, nil)
}

func (store *Store) MarkRunning(ctx context.Context, toolCallID string, risk string) (*CallRecord, error) {
	return store.update(ctx, toolCallID, StatusRunning, risk, "", "", "", nil, nil)
}

func (store *Store) MarkCompleted(ctx context.Context, toolCallID string, outputPreview string, artifactRefs []string, redactions []string) (*CallRecord, error) {
	return store.update(ctx, toolCallID, StatusCompleted, "", outputPreview, "", "", artifactRefs, redactions)
}

func (store *Store) MarkFailed(ctx context.Context, toolCallID string, outputPreview string, message string, redactions []string) (*CallRecord, error) {
	return store.update(ctx, toolCallID, StatusFailed, "", outputPreview, "", message, nil, redactions)
}

func (store *Store) update(ctx context.Context, toolCallID string, status string, risk string, outputPreview string, approvalID string, message string, artifactRefs []string, redactions []string) (*CallRecord, error) {
	if toolCallID == "" {
		return nil, fmt.Errorf("update tool call: tool_call_id is required")
	}
	current, err := store.Get(ctx, toolCallID)
	if err != nil {
		return nil, err
	}
	if risk == "" {
		risk = current.Risk
	}
	if outputPreview == "" {
		outputPreview = current.OutputPreview
	}
	if approvalID == "" {
		approvalID = current.ApprovalID
	}
	if artifactRefs == nil {
		artifactRefs = current.ArtifactRefs
	}
	if redactions == nil {
		redactions = current.Redactions
	}
	completedAt := current.CompletedAt
	if status == StatusCompleted || status == StatusFailed || status == StatusDenied {
		completedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := store.db.ExecContext(ctx, `
UPDATE tool_call_records
SET status = ?, risk = ?, output_preview = ?, artifact_refs_json = ?,
	approval_id = ?, error = ?, redactions_json = ?, updated_at = ?, completed_at = ?
WHERE tool_call_id = ?`,
		status,
		risk,
		outputPreview,
		mustJSON(artifactRefs),
		approvalID,
		message,
		mustJSON(redactions),
		now,
		completedAt,
		toolCallID,
	); err != nil {
		return nil, fmt.Errorf("update tool call: %w", err)
	}
	return store.Get(ctx, toolCallID)
}

func (store *Store) Get(ctx context.Context, toolCallID string) (*CallRecord, error) {
	row := store.db.QueryRowContext(ctx, `
SELECT tool_call_id, session_id, run_id, task_id, tool_id, status, risk,
	input_preview, output_preview, artifact_refs_json, approval_id, error,
	redactions_json, metadata_json, created_at, updated_at, completed_at
FROM tool_call_records
WHERE tool_call_id = ?`, toolCallID)
	return scanCall(row)
}

func (store *Store) ListBySession(ctx context.Context, sessionID string, limit int) ([]*CallRecord, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("list tool calls: session_id is required")
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := store.db.QueryContext(ctx, `
SELECT tool_call_id, session_id, run_id, task_id, tool_id, status, risk,
	input_preview, output_preview, artifact_refs_json, approval_id, error,
	redactions_json, metadata_json, created_at, updated_at, completed_at
FROM tool_call_records
WHERE session_id = ?
ORDER BY updated_at DESC
LIMIT ?`, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("list tool calls: %w", err)
	}
	defer rows.Close()
	var records []*CallRecord
	for rows.Next() {
		record, err := scanCall(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tool calls: %w", err)
	}
	return records, nil
}

type callScanner interface {
	Scan(dest ...any) error
}

func scanCall(scanner callScanner) (*CallRecord, error) {
	var record CallRecord
	var artifactRefsJSON string
	var redactionsJSON string
	var metadataJSON string
	var createdAt string
	var updatedAt string
	if err := scanner.Scan(
		&record.ToolCallID,
		&record.SessionID,
		&record.RunID,
		&record.TaskID,
		&record.ToolID,
		&record.Status,
		&record.Risk,
		&record.InputPreview,
		&record.OutputPreview,
		&artifactRefsJSON,
		&record.ApprovalID,
		&record.Error,
		&redactionsJSON,
		&metadataJSON,
		&createdAt,
		&updatedAt,
		&record.CompletedAt,
	); err != nil {
		return nil, fmt.Errorf("scan tool call: %w", err)
	}
	if err := json.Unmarshal([]byte(artifactRefsJSON), &record.ArtifactRefs); err != nil {
		return nil, fmt.Errorf("decode tool call artifacts: %w", err)
	}
	if err := json.Unmarshal([]byte(redactionsJSON), &record.Redactions); err != nil {
		return nil, fmt.Errorf("decode tool call redactions: %w", err)
	}
	var err error
	record.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse tool call created_at: %w", err)
	}
	record.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse tool call updated_at: %w", err)
	}
	record.Metadata = json.RawMessage(metadataJSON)
	return &record, nil
}

func mustJSON(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(payload)
}
