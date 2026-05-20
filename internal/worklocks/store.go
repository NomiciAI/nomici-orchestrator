package worklocks

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
	StatusHeld     = "held"
	StatusReleased = "released"
)

type Lock struct {
	LockID     string          `json:"lock_id"`
	RunID      string          `json:"run_id"`
	TaskID     string          `json:"task_id,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Resource   string          `json:"resource"`
	Mode       string          `json:"mode"`
	Status     string          `json:"status"`
	Owner      string          `json:"owner,omitempty"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	AcquiredAt time.Time       `json:"acquired_at"`
	ReleasedAt string          `json:"released_at,omitempty"`
}

type AcquireRequest struct {
	RunID      string
	TaskID     string
	ToolCallID string
	Resource   string
	Owner      string
	Metadata   json.RawMessage
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (store *Store) Acquire(ctx context.Context, request AcquireRequest) (*Lock, error) {
	if strings.TrimSpace(request.RunID) == "" {
		return nil, fmt.Errorf("acquire workspace lock: run_id is required")
	}
	request.Resource = strings.TrimSpace(request.Resource)
	if request.Resource == "" {
		return nil, fmt.Errorf("acquire workspace lock: resource is required")
	}
	if request.Owner == "" {
		request.Owner = "toolbroker"
	}
	if len(request.Metadata) == 0 {
		request.Metadata = json.RawMessage("{}")
	}
	now := time.Now().UTC()
	lock := &Lock{
		LockID:     ids.New("lock"),
		RunID:      request.RunID,
		TaskID:     request.TaskID,
		ToolCallID: request.ToolCallID,
		Resource:   request.Resource,
		Mode:       "exclusive",
		Status:     StatusHeld,
		Owner:      request.Owner,
		Metadata:   request.Metadata,
		AcquiredAt: now,
	}
	_, err := store.db.ExecContext(ctx, `
INSERT INTO workspace_locks (
	lock_id, run_id, task_id, tool_call_id, resource, mode, status, owner,
	metadata_json, acquired_at, released_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '')`,
		lock.LockID,
		lock.RunID,
		lock.TaskID,
		lock.ToolCallID,
		lock.Resource,
		lock.Mode,
		lock.Status,
		lock.Owner,
		string(lock.Metadata),
		lock.AcquiredAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, fmt.Errorf("workspace resource %q is locked", request.Resource)
		}
		return nil, fmt.Errorf("acquire workspace lock: %w", err)
	}
	return lock, nil
}

func (store *Store) Release(ctx context.Context, lockID string) error {
	if strings.TrimSpace(lockID) == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := store.db.ExecContext(ctx, `
UPDATE workspace_locks
SET status = ?, released_at = ?
WHERE lock_id = ? AND status = ?`, StatusReleased, now, lockID, StatusHeld)
	if err != nil {
		return fmt.Errorf("release workspace lock: %w", err)
	}
	return nil
}

func (store *Store) ListByRun(ctx context.Context, runID string, status string, limit int) ([]*Lock, error) {
	if strings.TrimSpace(runID) == "" {
		return nil, fmt.Errorf("list workspace locks: run_id is required")
	}
	if limit <= 0 {
		limit = 50
	}
	query := `
SELECT lock_id, run_id, task_id, tool_call_id, resource, mode, status, owner,
	metadata_json, acquired_at, released_at
FROM workspace_locks
WHERE run_id = ?`
	args := []any{runID}
	if strings.TrimSpace(status) != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY acquired_at DESC LIMIT ?"
	args = append(args, limit)
	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list workspace locks: %w", err)
	}
	defer rows.Close()
	locks := []*Lock{}
	for rows.Next() {
		lock, err := scanLock(rows)
		if err != nil {
			return nil, err
		}
		locks = append(locks, lock)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list workspace locks: %w", err)
	}
	return locks, nil
}

type lockScanner interface {
	Scan(dest ...any) error
}

func scanLock(scanner lockScanner) (*Lock, error) {
	var lock Lock
	var metadataJSON string
	var acquiredAt string
	if err := scanner.Scan(
		&lock.LockID,
		&lock.RunID,
		&lock.TaskID,
		&lock.ToolCallID,
		&lock.Resource,
		&lock.Mode,
		&lock.Status,
		&lock.Owner,
		&metadataJSON,
		&acquiredAt,
		&lock.ReleasedAt,
	); err != nil {
		return nil, fmt.Errorf("scan workspace lock: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, acquiredAt)
	if err != nil {
		return nil, fmt.Errorf("parse workspace lock acquired_at: %w", err)
	}
	lock.AcquiredAt = parsed
	lock.Metadata = json.RawMessage(metadataJSON)
	return &lock, nil
}
