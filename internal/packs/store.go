package packs

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

type Installation struct {
	PackID      string    `json:"pack_id"`
	Version     string    `json:"version"`
	Kind        string    `json:"kind"`
	Trust       string    `json:"trust"`
	ConfigPath  string    `json:"config_path"`
	Entrypoints []string  `json:"entrypoints"`
	InstalledAt time.Time `json:"installed_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (store *Store) SaveInstallation(ctx context.Context, installation *Installation) error {
	if installation == nil {
		return fmt.Errorf("save pack installation: nil installation")
	}
	if installation.PackID == "" {
		return fmt.Errorf("save pack installation: pack_id is required")
	}
	if installation.InstalledAt.IsZero() {
		installation.InstalledAt = time.Now().UTC()
	}
	if installation.UpdatedAt.IsZero() {
		installation.UpdatedAt = installation.InstalledAt
	}
	entrypoints, err := json.Marshal(installation.Entrypoints)
	if err != nil {
		return fmt.Errorf("marshal pack entrypoints: %w", err)
	}

	_, err = store.db.ExecContext(ctx, `
INSERT INTO pack_installations (
	pack_id, version, kind, trust, config_path, entrypoints_json, installed_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(pack_id) DO UPDATE SET
	version = excluded.version,
	kind = excluded.kind,
	trust = excluded.trust,
	config_path = excluded.config_path,
	entrypoints_json = excluded.entrypoints_json,
	updated_at = excluded.updated_at`,
		installation.PackID,
		installation.Version,
		installation.Kind,
		installation.Trust,
		installation.ConfigPath,
		string(entrypoints),
		installation.InstalledAt.Format(time.RFC3339Nano),
		installation.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("save pack installation: %w", err)
	}
	return nil
}

func (store *Store) ListInstallations(ctx context.Context) ([]*Installation, error) {
	rows, err := store.db.QueryContext(ctx, `
SELECT pack_id, version, kind, trust, config_path, entrypoints_json, installed_at, updated_at
FROM pack_installations
ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list pack installations: %w", err)
	}
	defer rows.Close()

	var installations []*Installation
	for rows.Next() {
		installation, err := scanInstallation(rows)
		if err != nil {
			return nil, err
		}
		installations = append(installations, installation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list pack installations: %w", err)
	}
	return installations, nil
}

func (store *Store) GetInstallation(ctx context.Context, packID string) (*Installation, error) {
	row := store.db.QueryRowContext(ctx, `
SELECT pack_id, version, kind, trust, config_path, entrypoints_json, installed_at, updated_at
FROM pack_installations
WHERE pack_id = ?`, packID)
	return scanInstallation(row)
}

type installationScanner interface {
	Scan(dest ...any) error
}

func scanInstallation(row installationScanner) (*Installation, error) {
	var installation Installation
	var entrypointsJSON string
	var installedAt string
	var updatedAt string
	if err := row.Scan(
		&installation.PackID,
		&installation.Version,
		&installation.Kind,
		&installation.Trust,
		&installation.ConfigPath,
		&entrypointsJSON,
		&installedAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(entrypointsJSON), &installation.Entrypoints); err != nil {
		return nil, fmt.Errorf("decode pack entrypoints: %w", err)
	}
	parsedInstalledAt, err := time.Parse(time.RFC3339Nano, installedAt)
	if err != nil {
		return nil, fmt.Errorf("parse pack installed_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse pack updated_at: %w", err)
	}
	installation.InstalledAt = parsedInstalledAt
	installation.UpdatedAt = parsedUpdatedAt
	return &installation, nil
}
