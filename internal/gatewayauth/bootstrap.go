package gatewayauth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const BootstrapFile = "gateway.bootstrap.json"

type BootstrapRecord struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func BootstrapPathForDB(dbPath string) string {
	return filepath.Join(filepath.Dir(TokenPathForDB(dbPath)), BootstrapFile)
}

func CreateBootstrap(path string, ttl time.Duration) (BootstrapRecord, error) {
	token, err := Generate()
	if err != nil {
		return BootstrapRecord{}, err
	}
	record := BootstrapRecord{
		Token:     token,
		ExpiresAt: time.Now().UTC().Add(ttl),
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return BootstrapRecord{}, err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return BootstrapRecord{}, err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return BootstrapRecord{}, err
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o600); err != nil {
		return BootstrapRecord{}, err
	}
	return record, nil
}

func ConsumeBootstrap(path string, provided string, now time.Time) (bool, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	var record BootstrapRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return false, err
	}
	if !record.ExpiresAt.IsZero() && now.UTC().After(record.ExpiresAt) {
		_ = os.Remove(path)
		return false, nil
	}
	if !Matches(record.Token, provided) {
		return false, nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, nil
}
