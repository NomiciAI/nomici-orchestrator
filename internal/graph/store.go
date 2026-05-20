package graph

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (store *Store) Save(ctx context.Context, snapshot *Snapshot) error {
	payload, err := json.Marshal(snapshot.IR)
	if err != nil {
		return fmt.Errorf("marshal graph IR: %w", err)
	}
	_, err = store.db.ExecContext(ctx, `
INSERT INTO graph_snapshots (
	snapshot_id, schema_version, project_id, source_hash, ir_json, created_at
) VALUES (?, ?, ?, ?, ?, ?)`,
		snapshot.SnapshotID,
		snapshot.SchemaVersion,
		snapshot.ProjectID,
		snapshot.SourceHash,
		string(payload),
		snapshot.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("save graph snapshot: %w", err)
	}
	return nil
}

func (store *Store) Latest(ctx context.Context) (*Snapshot, error) {
	row := store.db.QueryRowContext(ctx, `
SELECT snapshot_id, schema_version, project_id, source_hash, ir_json, created_at
FROM graph_snapshots
ORDER BY created_at DESC
LIMIT 1`)
	return scanSnapshot(row)
}

func (store *Store) Get(ctx context.Context, snapshotID string) (*Snapshot, error) {
	if snapshotID == "" {
		return nil, fmt.Errorf("graph snapshot id is required")
	}
	row := store.db.QueryRowContext(ctx, `
SELECT snapshot_id, schema_version, project_id, source_hash, ir_json, created_at
FROM graph_snapshots
WHERE snapshot_id = ?`, snapshotID)
	return scanSnapshot(row)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSnapshot(row rowScanner) (*Snapshot, error) {
	var snapshot Snapshot
	var irJSON string
	var createdAt string
	if err := row.Scan(
		&snapshot.SnapshotID,
		&snapshot.SchemaVersion,
		&snapshot.ProjectID,
		&snapshot.SourceHash,
		&irJSON,
		&createdAt,
	); err != nil {
		return nil, err
	}
	parsedTime, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse graph snapshot time: %w", err)
	}
	snapshot.CreatedAt = parsedTime
	if err := json.Unmarshal([]byte(irJSON), &snapshot.IR); err != nil {
		return nil, fmt.Errorf("decode graph IR: %w", err)
	}
	return &snapshot, nil
}
