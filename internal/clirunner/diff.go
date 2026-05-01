package clirunner

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type workspaceSnapshot struct {
	Git   bool
	Files map[string]string
}

func captureWorkspace(workspace string) (*workspaceSnapshot, string, error) {
	if isGitWorkspace(workspace) {
		diff, err := captureGitDiff(workspace)
		if err != nil {
			return nil, "", err
		}
		return &workspaceSnapshot{Git: true}, diff, nil
	}

	manifest, err := fileManifest(workspace)
	if err != nil {
		return nil, "", err
	}
	return &workspaceSnapshot{Files: manifest}, renderManifest(manifest), nil
}

func captureWorkspaceDiff(workspace string, before *workspaceSnapshot) (*workspaceSnapshot, string, []string, error) {
	if before != nil && before.Git {
		diff, err := captureGitDiff(workspace)
		if err != nil {
			return nil, "", nil, err
		}
		changed, err := gitChangedFiles(workspace)
		if err != nil {
			return nil, "", nil, err
		}
		return &workspaceSnapshot{Git: true}, diff, changed, nil
	}

	after, err := fileManifest(workspace)
	if err != nil {
		return nil, "", nil, err
	}
	if before == nil {
		before = &workspaceSnapshot{Files: map[string]string{}}
	}
	diff, changed := manifestDiff(before.Files, after)
	return &workspaceSnapshot{Files: after}, diff, changed, nil
}

func isGitWorkspace(workspace string) bool {
	command := exec.Command("git", "-C", workspace, "rev-parse", "--is-inside-work-tree")
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &bytes.Buffer{}
	if err := command.Run(); err != nil {
		return false
	}
	return strings.TrimSpace(output.String()) == "true"
}

func captureGitDiff(workspace string) (string, error) {
	diff, err := gitOutput(workspace, "diff", "--binary", "--no-ext-diff", "--", ".", ":(exclude).nomici")
	if err != nil {
		return "", err
	}
	untracked, err := gitOutput(workspace, "ls-files", "--others", "--exclude-standard", "--", ".", ":(exclude).nomici")
	if err != nil {
		return "", err
	}
	untracked = strings.TrimSpace(untracked)
	if untracked == "" {
		return diff, nil
	}

	var builder strings.Builder
	builder.WriteString(diff)
	if diff != "" && !strings.HasSuffix(diff, "\n") {
		builder.WriteByte('\n')
	}
	builder.WriteString("# Untracked files\n")
	for _, line := range strings.Split(untracked, "\n") {
		path := strings.TrimSpace(line)
		if path == "" {
			continue
		}
		builder.WriteString("A ")
		builder.WriteString(filepath.ToSlash(path))
		builder.WriteByte('\n')
	}
	return builder.String(), nil
}

func gitOutput(workspace string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", workspace}, args...)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func gitChangedFiles(workspace string) ([]string, error) {
	output, err := gitOutput(workspace, "status", "--porcelain", "--", ".", ":(exclude).nomici")
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if renameParts := strings.Split(path, " -> "); len(renameParts) == 2 {
			path = renameParts[1]
		}
		seen[filepath.ToSlash(path)] = struct{}{}
	}
	return sortedKeys(seen), nil
}

func fileManifest(workspace string) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(workspace, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == workspace {
			return nil
		}
		rel, err := filepath.Rel(workspace, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if rel == ".nomici" || strings.HasPrefix(rel, ".nomici/") {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		hash, err := hashFile(path)
		if err != nil {
			return err
		}
		files[rel] = hash
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("capture workspace manifest: %w", err)
	}
	return files, nil
}

func renderManifest(manifest map[string]string) string {
	keys := make([]string, 0, len(manifest))
	for key := range manifest {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(manifest[key])
		builder.WriteString("  ")
		builder.WriteString(key)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func manifestDiff(before map[string]string, after map[string]string) (string, []string) {
	if before == nil {
		before = map[string]string{}
	}
	seen := map[string]struct{}{}
	for path := range before {
		seen[path] = struct{}{}
	}
	for path := range after {
		seen[path] = struct{}{}
	}

	paths := sortedKeys(seen)
	var builder strings.Builder
	var changed []string
	for _, path := range paths {
		beforeHash, beforeOK := before[path]
		afterHash, afterOK := after[path]
		switch {
		case !beforeOK && afterOK:
			builder.WriteString("A ")
			builder.WriteString(path)
			builder.WriteByte('\n')
			changed = append(changed, path)
		case beforeOK && !afterOK:
			builder.WriteString("D ")
			builder.WriteString(path)
			builder.WriteByte('\n')
			changed = append(changed, path)
		case beforeHash != afterHash:
			builder.WriteString("M ")
			builder.WriteString(path)
			builder.WriteByte('\n')
			changed = append(changed, path)
		}
	}
	return builder.String(), changed
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
