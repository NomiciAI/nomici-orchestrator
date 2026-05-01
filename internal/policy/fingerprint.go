package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
)

func Fingerprint(action ActionRequest) string {
	workspace := action.Workspace
	if absolute, err := filepath.Abs(workspace); err == nil {
		workspace = absolute
	}
	key := strings.Join([]string{
		action.ProjectID,
		action.ActionType,
		filepath.Clean(workspace),
	}, "\x00")
	sum := sha256.Sum256([]byte(key))
	return "sha256:" + hex.EncodeToString(sum[:])
}
