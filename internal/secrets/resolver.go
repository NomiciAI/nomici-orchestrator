package secrets

import (
	"os"
	"path/filepath"
	"strings"
)

// Resolver resolves secret references to their values.
type Resolver struct {
	localSecretPaths []string
}

func NewResolver() *Resolver {
	return &Resolver{localSecretPaths: []string{filepath.Join(".nomici", "secrets.env")}}
}

func NewResolverForConfig(configPath string) *Resolver {
	resolver := NewResolver()
	configDir := filepath.Dir(strings.TrimSpace(configPath))
	if configDir != "" && configDir != "." {
		resolver.localSecretPaths = append([]string{filepath.Join(configDir, ".nomici", "secrets.env")}, resolver.localSecretPaths...)
	}
	return resolver
}

func (resolver *Resolver) ResolveEnv(name string) (string, bool) {
	if name == "" {
		return "", false
	}
	if value, ok := os.LookupEnv(name); ok {
		return value, true
	}
	return resolver.lookupLocalSecret(name)
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

func (resolver *Resolver) lookupLocalSecret(name string) (string, bool) {
	for _, path := range resolver.localSecretPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if ok && strings.TrimSpace(key) == name {
				return strings.TrimSpace(value), true
			}
		}
	}
	return "", false
}
