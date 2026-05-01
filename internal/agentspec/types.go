package agentspec

type Spec struct {
	Version    string                    `yaml:"version" json:"version"`
	Project    Project                   `yaml:"project" json:"project"`
	Models     map[string]Model          `yaml:"models,omitempty" json:"models,omitempty"`
	Runtimes   map[string]Runtime        `yaml:"runtimes,omitempty" json:"runtimes,omitempty"`
	Agents     map[string]Agent          `yaml:"agents,omitempty" json:"agents,omitempty"`
	Tools      map[string]map[string]any `yaml:"tools,omitempty" json:"tools,omitempty"`
	MCP        map[string]any            `yaml:"mcp,omitempty" json:"mcp,omitempty"`
	Edges      []Edge                    `yaml:"edges,omitempty" json:"edges,omitempty"`
	Policies   map[string]any            `yaml:"policies,omitempty" json:"policies,omitempty"`
	Budgets    map[string]any            `yaml:"budgets,omitempty" json:"budgets,omitempty"`
	Deployment map[string]any            `yaml:"deployment,omitempty" json:"deployment,omitempty"`
	Profiles   map[string]any            `yaml:"profiles,omitempty" json:"profiles,omitempty"`
	Extensions map[string]any            `yaml:"extensions,omitempty" json:"extensions,omitempty"`
}

type Project struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

type Model struct {
	Kind          string   `yaml:"kind" json:"kind"`
	BaseURL       string   `yaml:"base_url" json:"base_url"`
	APIKeyEnv     string   `yaml:"api_key_env,omitempty" json:"api_key_env,omitempty"`
	Model         string   `yaml:"model" json:"model"`
	Capabilities  []string `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
	ContextWindow int      `yaml:"context_window,omitempty" json:"context_window,omitempty"`
}

type Runtime struct {
	Kind           string            `yaml:"kind" json:"kind"`
	Runner         string            `yaml:"runner,omitempty" json:"runner,omitempty"`
	Workspace      string            `yaml:"workspace,omitempty" json:"workspace,omitempty"`
	Invoke         RuntimeInvoke     `yaml:"invoke,omitempty" json:"invoke,omitempty"`
	Env            map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	EnvFrom        []string          `yaml:"env_from,omitempty" json:"env_from,omitempty"`
	Capabilities   map[string]any    `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
	Trust          string            `yaml:"trust,omitempty" json:"trust,omitempty"`
	TimeoutSeconds int               `yaml:"timeout_seconds,omitempty" json:"timeout_seconds,omitempty"`
}

type RuntimeInvoke struct {
	Executable string   `yaml:"executable,omitempty" json:"executable,omitempty"`
	Args       []string `yaml:"args,omitempty" json:"args,omitempty"`
	Stdin      string   `yaml:"stdin,omitempty" json:"stdin,omitempty"`
}

type Agent struct {
	Kind         string   `yaml:"kind" json:"kind"`
	Model        string   `yaml:"model,omitempty" json:"model,omitempty"`
	Runtime      string   `yaml:"runtime,omitempty" json:"runtime,omitempty"`
	Endpoint     string   `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Role         string   `yaml:"role,omitempty" json:"role,omitempty"`
	Instructions string   `yaml:"instructions,omitempty" json:"instructions,omitempty"`
	Tools        []string `yaml:"tools,omitempty" json:"tools,omitempty"`
	Subagents    []string `yaml:"subagents,omitempty" json:"subagents,omitempty"`
	Trust        string   `yaml:"trust,omitempty" json:"trust,omitempty"`
}

type Edge struct {
	From string `yaml:"from" json:"from"`
	To   string `yaml:"to" json:"to"`
	Mode string `yaml:"mode" json:"mode"`
}

type Source struct {
	File string `json:"file"`
	Path string `json:"path"`
	Line int    `json:"line,omitempty"`
}

type ValidationError struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
	Source      Source `json:"source"`
}

func (err ValidationError) Error() string {
	if err.Source.Line > 0 {
		return err.Source.File + ":" + itoa(err.Source.Line) + " " + err.Source.Path + ": " + err.Message
	}
	return err.Source.File + " " + err.Source.Path + ": " + err.Message
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[i:])
}
