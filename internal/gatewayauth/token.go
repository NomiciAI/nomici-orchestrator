package gatewayauth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	TokenEnv     = "NOMICI_GATEWAY_TOKEN"
	TokenFileEnv = "NOMICI_GATEWAY_TOKEN_FILE"
	TokenFile    = "gateway.token"
)

func TokenPathForDB(dbPath string) string {
	if envPath := strings.TrimSpace(os.Getenv(TokenFileEnv)); envPath != "" {
		return envPath
	}
	if strings.TrimSpace(dbPath) == "" {
		dbPath = filepath.Join(".nomici", "state.db")
	}
	return filepath.Join(filepath.Dir(dbPath), TokenFile)
}

func LoadForClient(dbPath string) (string, error) {
	if token := strings.TrimSpace(os.Getenv(TokenEnv)); token != "" {
		return token, nil
	}
	token, err := Read(TokenPathForDB(dbPath))
	if err != nil {
		return "", fmt.Errorf("Gateway token is not available. Remediation: run `nomici gateway run` or set %s: %w", TokenEnv, err)
	}
	return token, nil
}

func LoadOrCreate(path string) (string, bool, error) {
	if token := strings.TrimSpace(os.Getenv(TokenEnv)); token != "" {
		return token, false, nil
	}
	if token, err := Read(path); err == nil {
		return token, false, nil
	} else if !os.IsNotExist(err) {
		return "", false, err
	}

	token, err := Generate()
	if err != nil {
		return "", false, err
	}
	if err := Write(path, token); err != nil {
		return "", false, err
	}
	return token, true, nil
}

func Rotate(path string) (string, error) {
	token, err := Generate()
	if err != nil {
		return "", err
	}
	if err := Write(path, token); err != nil {
		return "", err
	}
	return token, nil
}

func Read(path string) (string, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(payload))
	if token == "" {
		return "", fmt.Errorf("gateway token file %s is empty", path)
	}
	return token, nil
}

func Write(path string, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("gateway token is empty")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create gateway token directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("secure gateway token directory: %w", err)
	}
	return os.WriteFile(path, []byte(token+"\n"), 0o600)
}

func Generate() (string, error) {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate gateway token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes[:]), nil
}

func Matches(expected string, provided string) bool {
	expected = strings.TrimSpace(expected)
	provided = strings.TrimSpace(provided)
	if expected == "" || provided == "" {
		return false
	}
	if len(expected) != len(provided) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) == 1
}
