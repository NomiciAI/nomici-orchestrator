package chats

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
)

const (
	ThreadStatusActive = "active"
	RoleUser           = "user"
	RoleAssistant      = "assistant"
	RoleSystem         = "system"
)

type Store struct {
	db *sql.DB
}

type Thread struct {
	ChatID    string          `json:"chat_id"`
	Title     string          `json:"title"`
	Status    string          `json:"status"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

type Message struct {
	MessageID string          `json:"message_id"`
	ChatID    string          `json:"chat_id"`
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	RunID     string          `json:"run_id,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

type Feedback struct {
	FeedbackID string          `json:"feedback_id"`
	ChatID     string          `json:"chat_id"`
	MessageID  string          `json:"message_id"`
	Score      string          `json:"score"`
	Note       string          `json:"note,omitempty"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

type Detail struct {
	Thread   *Thread    `json:"thread"`
	Messages []*Message `json:"messages"`
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (store *Store) CreateThread(ctx context.Context, title string, metadata json.RawMessage) (*Thread, error) {
	now := time.Now().UTC()
	if title == "" {
		title = "New chat"
	}
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}
	thread := &Thread{
		ChatID:    ids.New("chat"),
		Title:     title,
		Status:    ThreadStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  metadata,
	}
	_, err := store.db.ExecContext(ctx, `
INSERT INTO chat_threads (chat_id, title, status, created_at, updated_at, metadata_json)
VALUES (?, ?, ?, ?, ?, ?)`,
		thread.ChatID,
		thread.Title,
		thread.Status,
		thread.CreatedAt.Format(time.RFC3339Nano),
		thread.UpdatedAt.Format(time.RFC3339Nano),
		string(thread.Metadata),
	)
	if err != nil {
		return nil, fmt.Errorf("create chat thread: %w", err)
	}
	return thread, nil
}

func (store *Store) AddMessage(ctx context.Context, message *Message) (*Message, error) {
	if message.ChatID == "" {
		return nil, fmt.Errorf("add chat message: chat_id is required")
	}
	if message.Role == "" {
		return nil, fmt.Errorf("add chat message: role is required")
	}
	now := time.Now().UTC()
	if message.MessageID == "" {
		message.MessageID = ids.New("msg")
	}
	if len(message.Metadata) == 0 {
		message.Metadata = json.RawMessage("{}")
	}
	message.CreatedAt = now
	_, err := store.db.ExecContext(ctx, `
INSERT INTO chat_messages (message_id, chat_id, role, content, run_id, session_id, created_at, metadata_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		message.MessageID,
		message.ChatID,
		message.Role,
		message.Content,
		message.RunID,
		message.SessionID,
		message.CreatedAt.Format(time.RFC3339Nano),
		string(message.Metadata),
	)
	if err != nil {
		return nil, fmt.Errorf("add chat message: %w", err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE chat_threads SET updated_at = ? WHERE chat_id = ?`, now.Format(time.RFC3339Nano), message.ChatID); err != nil {
		return nil, fmt.Errorf("touch chat thread: %w", err)
	}
	return message, nil
}

func (store *Store) UpdateMessageRun(ctx context.Context, messageID string, runID string, sessionID string) error {
	if messageID == "" {
		return fmt.Errorf("update chat message run: message_id is required")
	}
	_, err := store.db.ExecContext(ctx, `
UPDATE chat_messages
SET run_id = ?, session_id = ?
WHERE message_id = ?`, runID, sessionID, messageID)
	if err != nil {
		return fmt.Errorf("update chat message run: %w", err)
	}
	return nil
}

func (store *Store) UpsertFeedback(ctx context.Context, feedback *Feedback) (*Feedback, error) {
	if feedback == nil {
		return nil, fmt.Errorf("upsert chat feedback: feedback is required")
	}
	if feedback.ChatID == "" {
		return nil, fmt.Errorf("upsert chat feedback: chat_id is required")
	}
	if feedback.MessageID == "" {
		return nil, fmt.Errorf("upsert chat feedback: message_id is required")
	}
	if feedback.Score == "" {
		return nil, fmt.Errorf("upsert chat feedback: score is required")
	}
	if len(feedback.Metadata) == 0 {
		feedback.Metadata = json.RawMessage("{}")
	}
	now := time.Now().UTC()
	existing := store.db.QueryRowContext(ctx, `SELECT feedback_id, created_at FROM chat_feedback WHERE message_id = ?`, feedback.MessageID)
	var feedbackID string
	var createdAt string
	switch err := existing.Scan(&feedbackID, &createdAt); {
	case err == nil:
		feedback.FeedbackID = feedbackID
		parsed, parseErr := time.Parse(time.RFC3339Nano, createdAt)
		if parseErr != nil {
			return nil, fmt.Errorf("parse feedback created_at: %w", parseErr)
		}
		feedback.CreatedAt = parsed
	case err == sql.ErrNoRows:
		feedback.FeedbackID = ids.New("feedback")
		feedback.CreatedAt = now
	default:
		return nil, fmt.Errorf("lookup chat feedback: %w", err)
	}
	feedback.UpdatedAt = now
	_, err := store.db.ExecContext(ctx, `
INSERT INTO chat_feedback (feedback_id, chat_id, message_id, score, note, metadata_json, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(message_id) DO UPDATE SET
	score = excluded.score,
	note = excluded.note,
	metadata_json = excluded.metadata_json,
	updated_at = excluded.updated_at`,
		feedback.FeedbackID,
		feedback.ChatID,
		feedback.MessageID,
		feedback.Score,
		feedback.Note,
		string(feedback.Metadata),
		feedback.CreatedAt.Format(time.RFC3339Nano),
		feedback.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("upsert chat feedback: %w", err)
	}
	return feedback, nil
}

func (store *Store) ListThreads(ctx context.Context, limit int) ([]*Thread, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := store.db.QueryContext(ctx, `
SELECT chat_id, title, status, created_at, updated_at, metadata_json
FROM chat_threads
ORDER BY updated_at DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list chats: %w", err)
	}
	defer rows.Close()
	var threads []*Thread
	for rows.Next() {
		thread, err := scanThread(rows)
		if err != nil {
			return nil, err
		}
		threads = append(threads, thread)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list chats: %w", err)
	}
	return threads, nil
}

func (store *Store) GetThread(ctx context.Context, chatID string) (*Thread, error) {
	row := store.db.QueryRowContext(ctx, `
SELECT chat_id, title, status, created_at, updated_at, metadata_json
FROM chat_threads
WHERE chat_id = ?`, chatID)
	return scanThread(row)
}

func (store *Store) Detail(ctx context.Context, chatID string) (*Detail, error) {
	thread, err := store.GetThread(ctx, chatID)
	if err != nil {
		return nil, err
	}
	messages, err := store.ListMessages(ctx, chatID)
	if err != nil {
		return nil, err
	}
	return &Detail{Thread: thread, Messages: messages}, nil
}

func (store *Store) ListMessages(ctx context.Context, chatID string) ([]*Message, error) {
	rows, err := store.db.QueryContext(ctx, `
SELECT message_id, chat_id, role, content, run_id, session_id, created_at, metadata_json
FROM chat_messages
WHERE chat_id = ?
ORDER BY created_at, message_id`, chatID)
	if err != nil {
		return nil, fmt.Errorf("list chat messages: %w", err)
	}
	defer rows.Close()
	var messages []*Message
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list chat messages: %w", err)
	}
	return messages, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanThread(row scanner) (*Thread, error) {
	var thread Thread
	var createdAt string
	var updatedAt string
	var metadata string
	if err := row.Scan(&thread.ChatID, &thread.Title, &thread.Status, &createdAt, &updatedAt, &metadata); err != nil {
		return nil, fmt.Errorf("scan chat thread: %w", err)
	}
	var err error
	thread.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse chat created_at: %w", err)
	}
	thread.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse chat updated_at: %w", err)
	}
	thread.Metadata = json.RawMessage(metadata)
	return &thread, nil
}

func scanMessage(row scanner) (*Message, error) {
	var message Message
	var createdAt string
	var metadata string
	if err := row.Scan(&message.MessageID, &message.ChatID, &message.Role, &message.Content, &message.RunID, &message.SessionID, &createdAt, &metadata); err != nil {
		return nil, fmt.Errorf("scan chat message: %w", err)
	}
	var err error
	message.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse chat message created_at: %w", err)
	}
	message.Metadata = json.RawMessage(metadata)
	return &message, nil
}
