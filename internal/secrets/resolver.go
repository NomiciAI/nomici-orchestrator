package secrets

import (
	"os"
	"strings"
)

// Resolver resolves secret references to their values.
type Resolver struct{}

func NewResolver() *Resolver {
	return &Resolver{}
}

func (resolver *Resolver) ResolveEnv(name string) (string, bool) {
	if name == "" {
		return "", false
	}
	return os.LookupEnv(name)
}

func (resolver *Resolver) Redact(value string) string {
	if value == "" {
		return ""
	}
	if !LooksSensitive(value) {
		return value
	}
	return "[redacted]"
}

func RedactedEnv(name string) string {
	if name == "" {
		return "[redacted]"
	}
	return "[redacted:" + name + "]"
}

func LooksSensitive(value string) bool {
	if len(value) > 20 {
		return true
	}
	lower := strings.ToLower(value)
	return strings.HasPrefix(lower, "sk-") ||
		strings.HasPrefix(lower, "ant-") ||
		strings.HasPrefix(lower, "ghp_") ||
		strings.HasPrefix(lower, "xox")
}
