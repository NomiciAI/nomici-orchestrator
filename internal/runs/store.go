package runs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
)

const (
	SessionStatusRunning   = "running"
	SessionStatusCompleted = "completed"
	SessionStatusFailed    = "failed"
	SessionStatusCancelled = "cancelled"

	TaskStatusQueued             = "queued"
	TaskStatusRunning            = "running"
	TaskStatusWaitingForApproval = "waiting_for_approval"
	TaskStatusBlocked            = "blocked"
	TaskStatusNeedsClarification = "needs_clarification"
	TaskStatusPlanReview         = "plan_review"
	TaskStatusCompleted          = "completed"
	TaskStatusFailed             = "failed"
	TaskStatusCancelled          = "cancelled"
)

type Store struct {
	db *sql.DB
}

type Session struct {
	SessionID       string          `json:"session_id"`
	RunID           string          `json:"run_id"`
	ProjectID       string          `json:"project_id"`
	GraphSnapshotID string          `json:"graph_snapshot_id"`
	Title           string          `json:"title"`
	SourceChannel   string          `json:"source_channel"`
	Status          string          `json:"status"`
	StartedAt       time.Time       `json:"started_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
}

type Task struct {
	TaskID            string          `json:"task_id"`
	RunID             string          `json:"run_id"`
	ParentTaskID      string          `json:"parent_task_id,omitempty"`
	AgentID           string          `json:"agent_id"`
	RuntimeID         string          `json:"runtime_id,omitempty"`
	Status            string          `json:"status"`
	ContextSnapshotID string          `json:"context_snapshot_id,omitempty"`
	ArtifactRefs      []string        `json:"artifact_refs"`
	ApprovalRefs      []string        `json:"approval_refs"`
	StartedAt         time.Time       `json:"started_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	CompletedAt       *time.Time      `json:"completed_at,omitempty"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
}

type CreateSessionRequest struct {
	RunID           string
	ProjectID       string
	GraphSnapshotID string
	Title           string
	SourceChannel   string
	Status          string
	Metadata        json.RawMessage
}

type CreateTaskRequest struct {
	TaskID       string
	RunID        string
	ParentTaskID string
	AgentID      string
	RuntimeID    string
	Status       string
	Metadata     json.RawMessage
}

type SessionDetail struct {
	Session *Session `json:"session"`
	Tasks   []*Task  `json:"tasks"`
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (store *Store) CreateSession(ctx context.Context, request CreateSessionRequest) (*Session, error) {
	if request.RunID == "" {
		return nil, fmt.Errorf("create run session: run_id is required")
	}
	if request.ProjectID == "" {
		return nil, fmt.Errorf("create run session: project_id is required")
	}
	if request.GraphSnapshotID == "" {
		return nil, fmt.Errorf("create run session: graph_snapshot_id is required")
	}
	now := time.Now().UTC()
	session := &Session{
		SessionID:       ids.New("session"),
		RunID:           request.RunID,
		ProjectID:       request.ProjectID,
		GraphSnapshotID: request.GraphSnapshotID,
		Title:           request.Title,
		SourceChannel:   request.SourceChannel,
		Status:          request.Status,
		StartedAt:       now,
		UpdatedAt:       now,
		Metadata:        request.Metadata,
	}
	if session.Title == "" {
		session.Title = request.RunID
	}
	if session.SourceChannel == "" {
		session.SourceChannel = "console"
	}
	if session.Status == "" {
		session.Status = SessionStatusRunning
	}
	if len(session.Metadata) == 0 {
		session.Metadata = json.RawMessage("{}")
	}
	_, err := store.db.ExecContext(ctx, `
INSERT INTO run_sessions (
	session_id, run_id, project_id, graph_snapshot_id, title, source_channel,
	status, started_at, updated_at, completed_at, metadata_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?)`,
		session.SessionID,
		session.RunID,
		session.ProjectID,
		session.GraphSnapshotID,
		session.Title,
		session.SourceChannel,
		session.Status,
		session.StartedAt.Format(time.RFC3339Nano),
		session.UpdatedAt.Format(time.RFC3339Nano),
		string(session.Metadata),
	)
	if err != nil {
		return nil, fmt.Errorf("create run session: %w", err)
	}
	return session, nil
}

func (store *Store) CreateTask(ctx context.Context, request CreateTaskRequest) (*Task, error) {
	if request.RunID == "" {
		return nil, fmt.Errorf("create run task: run_id is required")
	}
	if request.AgentID == "" {
		return nil, fmt.Errorf("create run task: agent_id is required")
	}
	task := &Task{
		TaskID:       request.TaskID,
		RunID:        request.RunID,
		ParentTaskID: request.ParentTaskID,
		AgentID:      request.AgentID,
		RuntimeID:    request.RuntimeID,
		Status:       request.Status,
		StartedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		ArtifactRefs: []string{},
		ApprovalRefs: []string{},
		Metadata:     request.Metadata,
	}
	if task.TaskID == "" {
		task.TaskID = ids.New("task")
	}
	if task.Status == "" {
		task.Status = TaskStatusQueued
	}
	if len(task.Metadata) == 0 {
		task.Metadata = json.RawMessage("{}")
	}
	artifactRefs, err := json.Marshal(task.ArtifactRefs)
	if err != nil {
		return nil, err
	}
	approvalRefs, err := json.Marshal(task.ApprovalRefs)
	if err != nil {
		return nil, err
	}
	_, err = store.db.ExecContext(ctx, `
INSERT INTO run_tasks (
	task_id, run_id, parent_task_id, agent_id, runtime_id, status,
	context_snapshot_id, artifact_refs_json, approval_refs_json,
	started_at, updated_at, completed_at, metadata_json
) VALUES (?, ?, ?, ?, ?, ?, '', ?, ?, ?, ?, '', ?)`,
		task.TaskID,
		task.RunID,
		task.ParentTaskID,
		task.AgentID,
		task.RuntimeID,
		task.Status,
		string(artifactRefs),
		string(approvalRefs),
		task.StartedAt.Format(time.RFC3339Nano),
		task.UpdatedAt.Format(time.RFC3339Nano),
		string(task.Metadata),
	)
	if err != nil {
		return nil, fmt.Errorf("create run task: %w", err)
	}
	return task, nil
}

func (store *Store) CompleteSession(ctx context.Context, runID string, status string) error {
	if status == "" {
		status = SessionStatusCompleted
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := store.db.ExecContext(ctx, `
UPDATE run_sessions
SET status = ?, updated_at = ?, completed_at = ?
WHERE run_id = ?`, status, now, now, runID)
	if err != nil {
		return fmt.Errorf("complete run session: %w", err)
	}
	return nil
}

func (store *Store) CompleteTasks(ctx context.Context, runID string, status string) error {
	if status == "" {
		status = TaskStatusCompleted
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := store.db.ExecContext(ctx, `
UPDATE run_tasks
SET status = ?, updated_at = ?, completed_at = ?
WHERE run_id = ? AND status IN (?, ?)`, status, now, now, runID, TaskStatusQueued, TaskStatusRunning)
	if err != nil {
		return fmt.Errorf("complete run tasks: %w", err)
	}
	return nil
}

func (store *Store) CancelSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("cancel run session: session_id is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := store.db.ExecContext(ctx, `
UPDATE run_sessions
SET status = ?, updated_at = ?, completed_at = ?
WHERE session_id = ? AND status = ?`, SessionStatusCancelled, now, now, sessionID, SessionStatusRunning)
	if err != nil {
		return fmt.Errorf("cancel run session: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("cancel run session: %w", err)
	}
	if affected == 0 {
		existing, lookupErr := store.GetBySession(ctx, sessionID)
		if lookupErr != nil {
			return lookupErr
		}
		return fmt.Errorf("cancel run session: session is %s", existing.Session.Status)
	}
	return nil
}

func (store *Store) CancelTasks(ctx context.Context, runID string) error {
	if runID == "" {
		return fmt.Errorf("cancel run tasks: run_id is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := store.db.ExecContext(ctx, `
UPDATE run_tasks
SET status = ?, updated_at = ?, completed_at = ?
WHERE run_id = ? AND status IN (?, ?, ?, ?, ?)`,
		TaskStatusCancelled,
		now,
		now,
		runID,
		TaskStatusQueued,
		TaskStatusRunning,
		TaskStatusWaitingForApproval,
		TaskStatusBlocked,
		TaskStatusPlanReview,
	)
	if err != nil {
		return fmt.Errorf("cancel run tasks: %w", err)
	}
	return nil
}

func (store *Store) ListSessions(ctx context.Context, limit int) ([]*Session, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := store.db.QueryContext(ctx, `
SELECT session_id, run_id, project_id, graph_snapshot_id, title, source_channel,
	status, started_at, updated_at, completed_at, metadata_json
FROM run_sessions
ORDER BY updated_at DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list run sessions: %w", err)
	}
	defer rows.Close()
	var sessions []*Session
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list run sessions: %w", err)
	}
	return sessions, nil
}

func (store *Store) GetByRun(ctx context.Context, runID string) (*SessionDetail, error) {
	row := store.db.QueryRowContext(ctx, `
SELECT session_id, run_id, project_id, graph_snapshot_id, title, source_channel,
	status, started_at, updated_at, completed_at, metadata_json
FROM run_sessions
WHERE run_id = ?`, runID)
	session, err := scanSession(row)
	if err != nil {
		return nil, err
	}
	tasks, err := store.ListTasks(ctx, runID)
	if err != nil {
		return nil, err
	}
	return &SessionDetail{Session: session, Tasks: tasks}, nil
}

func (store *Store) GetBySession(ctx context.Context, sessionID string) (*SessionDetail, error) {
	row := store.db.QueryRowContext(ctx, `
SELECT session_id, run_id, project_id, graph_snapshot_id, title, source_channel,
	status, started_at, updated_at, completed_at, metadata_json
FROM run_sessions
WHERE session_id = ?`, sessionID)
	session, err := scanSession(row)
	if err != nil {
		return nil, err
	}
	tasks, err := store.ListTasks(ctx, session.RunID)
	if err != nil {
		return nil, err
	}
	return &SessionDetail{Session: session, Tasks: tasks}, nil
}

func (store *Store) ListTasksBySession(ctx context.Context, sessionID string) ([]*Task, error) {
	detail, err := store.GetBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return detail.Tasks, nil
}

func (store *Store) ListTasks(ctx context.Context, runID string) ([]*Task, error) {
	rows, err := store.db.QueryContext(ctx, `
SELECT task_id, run_id, parent_task_id, agent_id, runtime_id, status,
	context_snapshot_id, artifact_refs_json, approval_refs_json,
	started_at, updated_at, completed_at, metadata_json
FROM run_tasks
WHERE run_id = ?
ORDER BY started_at, task_id`, runID)
	if err != nil {
		return nil, fmt.Errorf("list run tasks: %w", err)
	}
	defer rows.Close()
	var tasks []*Task
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list run tasks: %w", err)
	}
	return tasks, nil
}

type sessionScanner interface {
	Scan(dest ...any) error
}

func scanSession(scanner sessionScanner) (*Session, error) {
	var session Session
	var startedAt string
	var updatedAt string
	var completedAt string
	var metadataJSON string
	if err := scanner.Scan(
		&session.SessionID,
		&session.RunID,
		&session.ProjectID,
		&session.GraphSnapshotID,
		&session.Title,
		&session.SourceChannel,
		&session.Status,
		&startedAt,
		&updatedAt,
		&completedAt,
		&metadataJSON,
	); err != nil {
		return nil, fmt.Errorf("scan run session: %w", err)
	}
	var err error
	session.StartedAt, err = time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return nil, fmt.Errorf("parse session started_at: %w", err)
	}
	session.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse session updated_at: %w", err)
	}
	if completedAt != "" {
		parsed, err := time.Parse(time.RFC3339Nano, completedAt)
		if err != nil {
			return nil, fmt.Errorf("parse session completed_at: %w", err)
		}
		session.CompletedAt = &parsed
	}
	session.Metadata = json.RawMessage(metadataJSON)
	return &session, nil
}

func scanTask(scanner sessionScanner) (*Task, error) {
	var task Task
	var artifactRefsJSON string
	var approvalRefsJSON string
	var startedAt string
	var updatedAt string
	var completedAt string
	var metadataJSON string
	if err := scanner.Scan(
		&task.TaskID,
		&task.RunID,
		&task.ParentTaskID,
		&task.AgentID,
		&task.RuntimeID,
		&task.Status,
		&task.ContextSnapshotID,
		&artifactRefsJSON,
		&approvalRefsJSON,
		&startedAt,
		&updatedAt,
		&completedAt,
		&metadataJSON,
	); err != nil {
		return nil, fmt.Errorf("scan run task: %w", err)
	}
	if err := json.Unmarshal([]byte(artifactRefsJSON), &task.ArtifactRefs); err != nil {
		return nil, fmt.Errorf("decode artifact refs: %w", err)
	}
	if err := json.Unmarshal([]byte(approvalRefsJSON), &task.ApprovalRefs); err != nil {
		return nil, fmt.Errorf("decode approval refs: %w", err)
	}
	var err error
	task.StartedAt, err = time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return nil, fmt.Errorf("parse task started_at: %w", err)
	}
	task.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse task updated_at: %w", err)
	}
	if completedAt != "" {
		parsed, err := time.Parse(time.RFC3339Nano, completedAt)
		if err != nil {
			return nil, fmt.Errorf("parse task completed_at: %w", err)
		}
		task.CompletedAt = &parsed
	}
	task.Metadata = json.RawMessage(metadataJSON)
	return &task, nil
}
