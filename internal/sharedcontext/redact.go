package sharedcontext

import (
	"regexp"
	"strings"
)

var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)sk-[a-z0-9_-]{8,}`),
	regexp.MustCompile(`(?i)ghp_[a-z0-9_]{8,}`),
	regexp.MustCompile(`(?i)xox[baprs]-[a-z0-9_-]{8,}`),
	regexp.MustCompile(`(?i)bearer\s+[a-z0-9._~+/=-]{12,}`),
	regexp.MustCompile(`(?is)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`),
}

func PrepareItem(item *Item) error {
	if item == nil {
		return ErrNilItem
	}
	if strings.TrimSpace(item.ProjectID) == "" {
		return ErrMissingProjectID
	}
	if strings.TrimSpace(item.Scope) == "" {
		item.Scope = ScopeRun
	}
	if strings.TrimSpace(item.Kind) == "" {
		item.Kind = KindRunSummary
	}
	if strings.TrimSpace(item.Confidence) == "" {
		item.Confidence = ConfidenceGenerated
	}
	if strings.TrimSpace(item.Sensitivity) == "" {
		item.Sensitivity = SensitivityNormal
	}
	if strings.TrimSpace(item.Status) == "" {
		item.Status = StatusActive
	}
	item.Title = RedactText(item.Title)
	item.Body = RedactText(item.Body)
	for index, value := range item.ArtifactRefs {
		item.ArtifactRefs[index] = RedactText(value)
	}
	return nil
}

func PrepareSnapshot(snapshot *Snapshot) error {
	if snapshot == nil {
		return ErrNilSnapshot
	}
	if strings.TrimSpace(snapshot.ProjectID) == "" {
		return ErrMissingProjectID
	}
	if strings.TrimSpace(snapshot.RunID) == "" {
		return ErrMissingRunID
	}
	snapshot.Summary = RedactText(snapshot.Summary)
	for index := range snapshot.Decisions {
		snapshot.Decisions[index].Title = RedactText(snapshot.Decisions[index].Title)
		snapshot.Decisions[index].Body = RedactText(snapshot.Decisions[index].Body)
	}
	for index, value := range snapshot.OpenIssues {
		snapshot.OpenIssues[index] = RedactText(value)
	}
	for index, value := range snapshot.Recommendations {
		snapshot.Recommendations[index] = RedactText(value)
	}
	for index, value := range snapshot.ArtifactRefs {
		snapshot.ArtifactRefs[index] = RedactText(value)
	}
	return nil
}

func RedactText(value string) string {
	if value == "" {
		return ""
	}
	for _, pattern := range sensitivePatterns {
		value = pattern.ReplaceAllString(value, "[redacted]")
	}
	return value
}
