package packs

import "time"

type Manifest struct {
	ID          string            `json:"id" yaml:"id"`
	Name        string            `json:"name" yaml:"name"`
	Version     string            `json:"version" yaml:"version"`
	Kind        string            `json:"kind" yaml:"kind"`
	Description string            `json:"description" yaml:"description"`
	Publisher   string            `json:"publisher,omitempty" yaml:"publisher,omitempty"`
	License     string            `json:"license,omitempty" yaml:"license,omitempty"`
	Requires    map[string]string `json:"requires,omitempty" yaml:"requires,omitempty"`
	Permissions Permissions       `json:"permissions" yaml:"permissions"`
	Agents      PackAgents        `json:"agents,omitempty" yaml:"agents,omitempty"`
	Trust       Trust             `json:"trust,omitempty" yaml:"trust,omitempty"`
}

type Permissions struct {
	Filesystem FilesystemPermissions `json:"filesystem,omitempty" yaml:"filesystem,omitempty"`
	Shell      ShellPermissions      `json:"shell,omitempty" yaml:"shell,omitempty"`
}

type FilesystemPermissions struct {
	Read  []string `json:"read,omitempty" yaml:"read,omitempty"`
	Write []string `json:"write,omitempty" yaml:"write,omitempty"`
}

type ShellPermissions struct {
	Mode string `json:"mode,omitempty" yaml:"mode,omitempty"`
}

type PackAgents struct {
	Entrypoints []string `json:"entrypoints,omitempty" yaml:"entrypoints,omitempty"`
	Includes    []string `json:"includes,omitempty" yaml:"includes,omitempty"`
	Optional    []string `json:"optional,omitempty" yaml:"optional,omitempty"`
}

type Trust struct {
	Level string `json:"level,omitempty" yaml:"level,omitempty"`
}

type InstallOptions struct {
	ConfigPath string
	DBPath     string
	ModelID    string
	Force      bool
}

type InstallResult struct {
	PackID      string
	ConfigPath  string
	ModelID     string
	Created     bool
	Updated     bool
	InstalledAt time.Time
}
