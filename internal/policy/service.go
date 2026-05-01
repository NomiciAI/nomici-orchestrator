package policy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (service *Service) Check(ctx context.Context, action ActionRequest) (*Decision, error) {
	if action.RunID == "" {
		return nil, fmt.Errorf("policy check requires run_id")
	}
	if action.ActionID == "" {
		action.ActionID = ids.New("action")
	}
	if action.ActionType == "" {
		action.ActionType = ActionCLIInvoke
	}
	if action.Summary == "" {
		action.Summary = defaultSummary(action)
	}
	risk, decision, reason := classify(action)
	fingerprint := Fingerprint(action)

	if decision == DecisionAllow {
		return &Decision{Decision: DecisionAllow, Risk: risk, Reason: reason, Fingerprint: fingerprint}, nil
	}
	if decision == DecisionDeny {
		return &Decision{Decision: DecisionDeny, Risk: risk, Reason: reason, Fingerprint: fingerprint}, nil
	}

	grant, err := service.findUsableGrant(ctx, fingerprint, action.RunID)
	if err != nil {
		return nil, err
	}
	if grant != nil {
		return &Decision{
			Decision:    DecisionAllow,
			Risk:        risk,
			Reason:      "approved by " + grant.ApprovalID,
			Fingerprint: fingerprint,
			ApprovalID:  grant.ApprovalID,
			Scope:       grant.Scope,
		}, nil
	}

	approval := &Approval{
		ApprovalID:        ids.New("approval"),
		RunID:             action.RunID,
		ActionID:          action.ActionID,
		ActionType:        action.ActionType,
		ActionFingerprint: fingerprint,
		Status:            StatusPending,
		Risk:              risk,
		Summary:           action.Summary,
		Subject:           action.Subject,
		RequestedByAgent:  action.AgentID,
		RuntimeID:         action.RuntimeID,
		Reason:            reason,
		RequestedAt:       time.Now().UTC(),
	}
	if err := service.saveApproval(ctx, approval); err != nil {
		return nil, err
	}
	return &Decision{
		Decision:    DecisionApproval,
		Risk:        risk,
		Reason:      reason,
		Fingerprint: fingerprint,
		ApprovalID:  approval.ApprovalID,
		RequestedAt: approval.RequestedAt,
	}, nil
}

func (service *Service) Grant(ctx context.Context, approvalID string, scope string) (*Approval, error) {
	if scope == "" {
		scope = ScopeOnce
	}
	if scope != ScopeOnce && scope != ScopeRun {
		return nil, fmt.Errorf("unsupported approval scope %q; use once or run", scope)
	}
	approval, err := service.Get(ctx, approvalID)
	if err != nil {
		return nil, err
	}
	if approval.Status != StatusPending {
		return nil, fmt.Errorf("approval %s is %s, not pending", approvalID, approval.Status)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := service.db.ExecContext(ctx, `
UPDATE approvals
SET status = ?, scope = ?, resolved_at = ?
WHERE approval_id = ?`, StatusGranted, scope, now, approvalID); err != nil {
		return nil, fmt.Errorf("grant approval: %w", err)
	}
	return service.Get(ctx, approvalID)
}

func (service *Service) Deny(ctx context.Context, approvalID string) (*Approval, error) {
	approval, err := service.Get(ctx, approvalID)
	if err != nil {
		return nil, err
	}
	if approval.Status != StatusPending {
		return nil, fmt.Errorf("approval %s is %s, not pending", approvalID, approval.Status)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := service.db.ExecContext(ctx, `
UPDATE approvals
SET status = ?, resolved_at = ?
WHERE approval_id = ?`, StatusDenied, now, approvalID); err != nil {
		return nil, fmt.Errorf("deny approval: %w", err)
	}
	return service.Get(ctx, approvalID)
}

func (service *Service) Get(ctx context.Context, approvalID string) (*Approval, error) {
	row := service.db.QueryRowContext(ctx, `
SELECT approval_id, run_id, action_id, action_type, action_fingerprint,
	status, risk, scope, summary, subject_json, requested_by_agent, runtime_id,
	reason, requested_at, resolved_at, consumed_at, bound_run_id
FROM approvals
WHERE approval_id = ?`, approvalID)
	return scanApproval(row)
}

func (service *Service) List(ctx context.Context, status string) ([]*Approval, error) {
	query := `
SELECT approval_id, run_id, action_id, action_type, action_fingerprint,
	status, risk, scope, summary, subject_json, requested_by_agent, runtime_id,
	reason, requested_at, resolved_at, consumed_at, bound_run_id
FROM approvals`
	args := []any{}
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY requested_at DESC"
	rows, err := service.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list approvals: %w", err)
	}
	defer rows.Close()

	var approvals []*Approval
	for rows.Next() {
		approval, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		approvals = append(approvals, approval)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list approvals: %w", err)
	}
	return approvals, nil
}

func (service *Service) findUsableGrant(ctx context.Context, fingerprint string, runID string) (*Approval, error) {
	rows, err := service.db.QueryContext(ctx, `
SELECT approval_id, run_id, action_id, action_type, action_fingerprint,
	status, risk, scope, summary, subject_json, requested_by_agent, runtime_id,
	reason, requested_at, resolved_at, consumed_at, bound_run_id
FROM approvals
WHERE action_fingerprint = ? AND status = ?
ORDER BY resolved_at ASC`, fingerprint, StatusGranted)
	if err != nil {
		return nil, fmt.Errorf("find approval grant: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		approval, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		switch approval.Scope {
		case ScopeOnce:
			if approval.ConsumedAt != "" {
				continue
			}
			now := time.Now().UTC().Format(time.RFC3339Nano)
			if _, err := service.db.ExecContext(ctx, "UPDATE approvals SET consumed_at = ? WHERE approval_id = ?", now, approval.ApprovalID); err != nil {
				return nil, fmt.Errorf("consume approval: %w", err)
			}
			approval.ConsumedAt = now
			return approval, nil
		case ScopeRun:
			if approval.BoundRunID == "" {
				if _, err := service.db.ExecContext(ctx, "UPDATE approvals SET bound_run_id = ? WHERE approval_id = ?", runID, approval.ApprovalID); err != nil {
					return nil, fmt.Errorf("bind run approval: %w", err)
				}
				approval.BoundRunID = runID
				return approval, nil
			}
			if approval.BoundRunID == runID {
				return approval, nil
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find approval grant: %w", err)
	}
	return nil, nil
}

func (service *Service) saveApproval(ctx context.Context, approval *Approval) error {
	_, err := service.db.ExecContext(ctx, `
INSERT INTO approvals (
	approval_id, run_id, action_id, action_type, action_fingerprint,
	status, risk, scope, summary, subject_json, requested_by_agent, runtime_id,
	reason, requested_at, resolved_at, consumed_at, bound_run_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		approval.ApprovalID,
		approval.RunID,
		approval.ActionID,
		approval.ActionType,
		approval.ActionFingerprint,
		approval.Status,
		approval.Risk,
		approval.Scope,
		approval.Summary,
		mustJSON(approval.Subject),
		approval.RequestedByAgent,
		approval.RuntimeID,
		approval.Reason,
		approval.RequestedAt.Format(time.RFC3339Nano),
		approval.ResolvedAt,
		approval.ConsumedAt,
		approval.BoundRunID,
	)
	if err != nil {
		return fmt.Errorf("save approval: %w", err)
	}
	return nil
}

func classify(action ActionRequest) (string, string, string) {
	if action.ActionType != ActionCLIInvoke {
		return RiskHigh, DecisionApproval, "unknown action requires approval"
	}
	if isCriticalWorkspace(action.Workspace) {
		return RiskCritical, DecisionDeny, "workspace targets a protected system path"
	}
	if action.FilesWrite {
		return RiskMedium, DecisionApproval, "mutable cli_agent workspace execution requires approval"
	}
	return RiskLow, DecisionAllow, "read-only cli_agent execution is allowed"
}

func isCriticalWorkspace(workspace string) bool {
	if workspace == "" {
		return false
	}
	absolute, err := filepath.Abs(workspace)
	if err != nil {
		return true
	}
	clean := filepath.Clean(absolute)
	if clean == string(filepath.Separator) {
		return true
	}
	if runtime.GOOS == "windows" {
		return false
	}
	protected := []string{"/bin", "/boot", "/dev", "/etc", "/private/etc", "/sbin", "/sys", "/System", "/usr/bin", "/usr/sbin"}
	for _, prefix := range protected {
		if clean == prefix || strings.HasPrefix(clean, prefix+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func defaultSummary(action ActionRequest) string {
	switch action.ActionType {
	case ActionCLIInvoke:
		if action.FilesWrite {
			return "Run mutable cli_agent in " + action.Workspace
		}
		return "Run read-only cli_agent in " + action.Workspace
	default:
		return action.ActionType
	}
}

type approvalScanner interface {
	Scan(dest ...any) error
}

func scanApproval(row approvalScanner) (*Approval, error) {
	var approval Approval
	var subjectJSON string
	var requestedAt string
	if err := row.Scan(
		&approval.ApprovalID,
		&approval.RunID,
		&approval.ActionID,
		&approval.ActionType,
		&approval.ActionFingerprint,
		&approval.Status,
		&approval.Risk,
		&approval.Scope,
		&approval.Summary,
		&subjectJSON,
		&approval.RequestedByAgent,
		&approval.RuntimeID,
		&approval.Reason,
		&requestedAt,
		&approval.ResolvedAt,
		&approval.ConsumedAt,
		&approval.BoundRunID,
	); err != nil {
		return nil, fmt.Errorf("scan approval: %w", err)
	}
	if err := json.Unmarshal([]byte(subjectJSON), &approval.Subject); err != nil {
		return nil, fmt.Errorf("decode approval subject: %w", err)
	}
	parsedTime, err := time.Parse(time.RFC3339Nano, requestedAt)
	if err != nil {
		return nil, fmt.Errorf("parse approval requested time: %w", err)
	}
	approval.RequestedAt = parsedTime
	return &approval, nil
}

func mustJSON(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(payload)
}
