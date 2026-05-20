package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
	"github.com/NomiciAI/nomici-orchestrator/internal/sharedcontext"
)

const (
	StatusProposed = "proposed"
	StatusApproved = "approved"
	StatusRejected = "rejected"
	StatusDeleted  = "deleted"
)

type Proposal struct {
	ProposalID   string          `json:"proposal_id"`
	ProjectID    string          `json:"project_id"`
	SessionID    string          `json:"session_id"`
	RunID        string          `json:"run_id"`
	SourceType   string          `json:"source_type"`
	Title        string          `json:"title"`
	Body         string          `json:"body"`
	Status       string          `json:"status"`
	ContextID    string          `json:"context_id,omitempty"`
	ArtifactRefs []string        `json:"artifact_refs,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type CreateRequest struct {
	ProjectID    string
	SessionID    string
	RunID        string
	SourceType   string
	Title        string
	Body         string
	ArtifactRefs []string
	Metadata     json.RawMessage
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (store *Store) Create(ctx context.Context, request CreateRequest) (*Proposal, error) {
	if request.ProjectID == "" || request.SessionID == "" || request.RunID == "" {
		return nil, fmt.Errorf("create memory proposal: project_id, session_id, and run_id are required")
	}
	if request.SourceType == "" {
		request.SourceType = "session_summary"
	}
	if request.Title == "" {
		request.Title = "Session memory"
	}
	if request.Body == "" {
		return nil, fmt.Errorf("create memory proposal: body is required")
	}
	if len(request.Metadata) == 0 {
		request.Metadata = json.RawMessage("{}")
	}
	now := time.Now().UTC()
	proposal := &Proposal{
		ProposalID:   ids.New("memory"),
		ProjectID:    request.ProjectID,
		SessionID:    request.SessionID,
		RunID:        request.RunID,
		SourceType:   request.SourceType,
		Title:        request.Title,
		Body:         request.Body,
		Status:       StatusProposed,
		ArtifactRefs: request.ArtifactRefs,
		Metadata:     request.Metadata,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_, err := store.db.ExecContext(ctx, `
INSERT INTO memory_proposals (
	proposal_id, project_id, session_id, run_id, source_type, title, body,
	status, context_id, artifact_refs_json, metadata_json, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(run_id, source_type) DO UPDATE SET
	title = excluded.title,
	body = excluded.body,
	artifact_refs_json = excluded.artifact_refs_json,
	metadata_json = excluded.metadata_json,
	updated_at = excluded.updated_at`,
		proposal.ProposalID,
		proposal.ProjectID,
		proposal.SessionID,
		proposal.RunID,
		proposal.SourceType,
		proposal.Title,
		proposal.Body,
		proposal.Status,
		proposal.ContextID,
		mustJSON(proposal.ArtifactRefs),
		string(proposal.Metadata),
		proposal.CreatedAt.Format(time.RFC3339Nano),
		proposal.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("create memory proposal: %w", err)
	}
	loaded, err := store.GetByRunSource(ctx, request.RunID, request.SourceType)
	if err != nil {
		return nil, err
	}
	return loaded, nil
}

func (store *Store) List(ctx context.Context, status string, limit int) ([]*Proposal, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
SELECT proposal_id, project_id, session_id, run_id, source_type, title, body,
	status, context_id, artifact_refs_json, metadata_json, created_at, updated_at
FROM memory_proposals`
	args := []any{}
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY updated_at DESC LIMIT ?"
	args = append(args, limit)
	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list memory proposals: %w", err)
	}
	defer rows.Close()
	var proposals []*Proposal
	for rows.Next() {
		proposal, err := scanProposal(rows)
		if err != nil {
			return nil, err
		}
		proposals = append(proposals, proposal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list memory proposals: %w", err)
	}
	return proposals, nil
}

func (store *Store) Get(ctx context.Context, proposalID string) (*Proposal, error) {
	row := store.db.QueryRowContext(ctx, `
SELECT proposal_id, project_id, session_id, run_id, source_type, title, body,
	status, context_id, artifact_refs_json, metadata_json, created_at, updated_at
FROM memory_proposals
WHERE proposal_id = ?`, proposalID)
	return scanProposal(row)
}

func (store *Store) GetByRunSource(ctx context.Context, runID string, sourceType string) (*Proposal, error) {
	row := store.db.QueryRowContext(ctx, `
SELECT proposal_id, project_id, session_id, run_id, source_type, title, body,
	status, context_id, artifact_refs_json, metadata_json, created_at, updated_at
FROM memory_proposals
WHERE run_id = ? AND source_type = ?`, runID, sourceType)
	return scanProposal(row)
}

func (store *Store) Approve(ctx context.Context, proposalID string, contextStore *sharedcontext.Store) (*Proposal, error) {
	proposal, err := store.Get(ctx, proposalID)
	if err != nil {
		return nil, err
	}
	if proposal.Status != StatusProposed {
		return nil, fmt.Errorf("memory proposal is %s", proposal.Status)
	}
	if contextStore == nil {
		return nil, fmt.Errorf("shared context store is not initialized")
	}
	item := &sharedcontext.Item{
		ProjectID:    proposal.ProjectID,
		RunID:        proposal.RunID,
		Scope:        sharedcontext.ScopeProject,
		Kind:         sharedcontext.KindRunSummary,
		Title:        proposal.Title,
		Body:         proposal.Body,
		Tags:         []string{"memory", proposal.SourceType},
		ArtifactRefs: proposal.ArtifactRefs,
		Source:       sharedcontext.Source{Kind: "memory_proposal", RunID: proposal.RunID},
		Confidence:   sharedcontext.ConfidenceGenerated,
		Sensitivity:  sharedcontext.SensitivityNormal,
		Status:       sharedcontext.StatusActive,
		Metadata:     map[string]string{"proposal_id": proposal.ProposalID},
	}
	if err := contextStore.SaveItem(ctx, item); err != nil {
		return nil, err
	}
	return store.setStatus(ctx, proposalID, StatusApproved, item.ContextID)
}

func (store *Store) Reject(ctx context.Context, proposalID string) (*Proposal, error) {
	return store.setStatus(ctx, proposalID, StatusRejected, "")
}

func (store *Store) Delete(ctx context.Context, proposalID string) (*Proposal, error) {
	return store.setStatus(ctx, proposalID, StatusDeleted, "")
}

func (store *Store) setStatus(ctx context.Context, proposalID string, status string, contextID string) (*Proposal, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := store.db.ExecContext(ctx, `
UPDATE memory_proposals
SET status = ?, context_id = CASE WHEN ? != '' THEN ? ELSE context_id END, updated_at = ?
WHERE proposal_id = ?`, status, contextID, contextID, now, proposalID)
	if err != nil {
		return nil, fmt.Errorf("update memory proposal: %w", err)
	}
	return store.Get(ctx, proposalID)
}

type proposalScanner interface {
	Scan(dest ...any) error
}

func scanProposal(scanner proposalScanner) (*Proposal, error) {
	var proposal Proposal
	var artifactRefsJSON string
	var metadataJSON string
	var createdAt string
	var updatedAt string
	if err := scanner.Scan(
		&proposal.ProposalID,
		&proposal.ProjectID,
		&proposal.SessionID,
		&proposal.RunID,
		&proposal.SourceType,
		&proposal.Title,
		&proposal.Body,
		&proposal.Status,
		&proposal.ContextID,
		&artifactRefsJSON,
		&metadataJSON,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan memory proposal: %w", err)
	}
	if err := json.Unmarshal([]byte(artifactRefsJSON), &proposal.ArtifactRefs); err != nil {
		return nil, fmt.Errorf("decode memory proposal artifacts: %w", err)
	}
	var err error
	proposal.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse memory proposal created_at: %w", err)
	}
	proposal.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse memory proposal updated_at: %w", err)
	}
	proposal.Metadata = json.RawMessage(metadataJSON)
	return &proposal, nil
}

func mustJSON(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(payload)
}
