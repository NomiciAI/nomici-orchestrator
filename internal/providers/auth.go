package providers

import (
	"regexp"
	"strings"
	"unicode"
)

var envVarPattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

func ValidEnvVarName(value string) bool {
	return envVarPattern.MatchString(strings.TrimSpace(value))
}

func LooksLikeRawSecret(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "sk-") ||
		strings.HasPrefix(lower, "sk_") ||
		strings.HasPrefix(value, "AIza") ||
		strings.HasPrefix(lower, "xox") {
		return true
	}
	if len(value) < 32 {
		return false
	}
	var letters, digits, symbols int
	for _, r := range value {
		switch {
		case unicode.IsLetter(r):
			letters++
		case unicode.IsDigit(r):
			digits++
		case r == '_' || r == '-' || r == '.':
			symbols++
		}
	}
	return letters > 12 && digits > 4 && symbols > 0 && !strings.Contains(value, " ")
}
