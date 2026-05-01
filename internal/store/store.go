package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const DefaultDBPath = ".nomici/state.db"

// Open opens or creates the SQLite database at path.
func Open(path string) (*sql.DB, error) {
	if path == "" {
		path = DefaultDBPath
	}

	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable sqlite WAL: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable sqlite foreign keys: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	return db, nil
}

// Migrate applies pending schema migrations.
func Migrate(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("migrate database: nil db")
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS schema_version (
	version INTEGER PRIMARY KEY,
	applied_at TEXT NOT NULL DEFAULT (datetime('now'))
)`); err != nil {
		return fmt.Errorf("create schema_version table: %w", err)
	}

	for _, migration := range migrations {
		var exists int
		if err := tx.QueryRow("SELECT COUNT(1) FROM schema_version WHERE version = ?", migration.Version).Scan(&exists); err != nil {
			return fmt.Errorf("check migration %d: %w", migration.Version, err)
		}
		if exists > 0 {
			continue
		}

		if _, err := tx.Exec(migration.SQL); err != nil {
			return fmt.Errorf("apply migration %d: %w", migration.Version, err)
		}
		if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", migration.Version); err != nil {
			return fmt.Errorf("record migration %d: %w", migration.Version, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}

	return nil
}
