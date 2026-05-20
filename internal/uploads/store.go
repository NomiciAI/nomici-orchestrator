package uploads

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
)

const StatusReady = "ready"

type Store struct {
	db *sql.DB
}

type Upload struct {
	UploadID    string          `json:"upload_id"`
	SessionID   string          `json:"session_id"`
	RunID       string          `json:"run_id"`
	TaskID      string          `json:"task_id,omitempty"`
	Filename    string          `json:"filename"`
	Path        string          `json:"path"`
	SizeBytes   int64           `json:"size_bytes"`
	ContentType string          `json:"content_type,omitempty"`
	Status      string          `json:"status"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

type CreateRequest struct {
	SessionID   string
	RunID       string
	TaskID      string
	Filename    string
	Path        string
	SizeBytes   int64
	ContentType string
	Status      string
	Metadata    json.RawMessage
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (store *Store) Create(ctx context.Context, request CreateRequest) (*Upload, error) {
	if request.SessionID == "" {
		return nil, fmt.Errorf("create upload: session_id is required")
	}
	if request.RunID == "" {
		return nil, fmt.Errorf("create upload: run_id is required")
	}
	if request.Filename == "" {
		return nil, fmt.Errorf("create upload: filename is required")
	}
	if request.Path == "" {
		return nil, fmt.Errorf("create upload: path is required")
	}
	if request.Status == "" {
		request.Status = StatusReady
	}
	if len(request.Metadata) == 0 {
		request.Metadata = json.RawMessage("{}")
	}
	now := time.Now().UTC()
	upload := &Upload{
		UploadID:    ids.New("upload"),
		SessionID:   request.SessionID,
		RunID:       request.RunID,
		TaskID:      request.TaskID,
		Filename:    request.Filename,
		Path:        request.Path,
		SizeBytes:   request.SizeBytes,
		ContentType: request.ContentType,
		Status:      request.Status,
		Metadata:    request.Metadata,
		CreatedAt:   now,
	}
	_, err := store.db.ExecContext(ctx, `
INSERT INTO upload_records (
	upload_id, session_id, run_id, task_id, filename, path, size_bytes,
	content_type, status, metadata_json, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		upload.UploadID,
		upload.SessionID,
		upload.RunID,
		upload.TaskID,
		upload.Filename,
		upload.Path,
		upload.SizeBytes,
		upload.ContentType,
		upload.Status,
		string(upload.Metadata),
		upload.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("create upload: %w", err)
	}
	return upload, nil
}

func (store *Store) List(ctx context.Context, sessionID string, limit int) ([]*Upload, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows *sql.Rows
	var err error
	if sessionID == "" {
		rows, err = store.db.QueryContext(ctx, `
SELECT upload_id, session_id, run_id, task_id, filename, path, size_bytes,
	content_type, status, metadata_json, created_at
FROM upload_records
ORDER BY created_at DESC
LIMIT ?`, limit)
	} else {
		rows, err = store.db.QueryContext(ctx, `
SELECT upload_id, session_id, run_id, task_id, filename, path, size_bytes,
	content_type, status, metadata_json, created_at
FROM upload_records
WHERE session_id = ?
ORDER BY created_at DESC
LIMIT ?`, sessionID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list uploads: %w", err)
	}
	defer rows.Close()
	var uploads []*Upload
	for rows.Next() {
		upload, err := scanUpload(rows)
		if err != nil {
			return nil, err
		}
		uploads = append(uploads, upload)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list uploads: %w", err)
	}
	return uploads, nil
}

type uploadScanner interface {
	Scan(dest ...any) error
}

func scanUpload(scanner uploadScanner) (*Upload, error) {
	var upload Upload
	var metadataJSON string
	var createdAt string
	if err := scanner.Scan(
		&upload.UploadID,
		&upload.SessionID,
		&upload.RunID,
		&upload.TaskID,
		&upload.Filename,
		&upload.Path,
		&upload.SizeBytes,
		&upload.ContentType,
		&upload.Status,
		&metadataJSON,
		&createdAt,
	); err != nil {
		return nil, fmt.Errorf("scan upload: %w", err)
	}
	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse upload created_at: %w", err)
	}
	upload.CreatedAt = parsedCreatedAt
	upload.Metadata = json.RawMessage(metadataJSON)
	return &upload, nil
}
