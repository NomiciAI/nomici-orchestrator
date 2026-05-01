package store

import (
	"path/filepath"
	"testing"
)

func TestOpenCreatesDatabaseFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".nomici", "state.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(1) FROM provider_profiles").Scan(&count); err != nil {
		t.Fatalf("query provider_profiles: %v", err)
	}
}
