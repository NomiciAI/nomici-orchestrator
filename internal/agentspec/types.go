package agentspec

type Spec struct {
	Version    string                    `yaml:"version" json:"version"`
	Project    Project                   `yaml:"project" json:"project"`
	Models     map[string]Model          `yaml:"models" json:"models,omitempty"`
	Runtimes   map[string]Runtime        `yaml:"runtimes" json:"runtimes,omitempty"`
	Agents     map[string]Agent          `yaml:"agents" json:"agents,omitempty"`
	Tools      map[string]map[string]any `yaml:"tools" json:"tools,omitempty"`
	MCP        map[string]any            `yaml:"mcp" json:"mcp,omitempty"`
	Edges      []Edge                    `yaml:"edges" json:"edges,omitempty"`
	Policies   map[string]any            `yaml:"policies" json:"policies,omitempty"`
	Budgets    map[string]any            `yaml:"budgets" json:"budgets,omitempty"`
	Deployment map[string]any            `yaml:"deployment" json:"deployment,omitempty"`
	Profiles   map[string]any            `yaml:"profiles" json:"profiles,omitempty"`
	Extensions map[string]any            `yaml:"extensions" json:"extensions,omitempty"`
}

type Project struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description,omitempty"`
}

type Model struct {
	Kind          string   `yaml:"kind" json:"kind"`
	BaseURL       string   `yaml:"base_url" json:"base_url"`
	APIKeyEnv     string   `yaml:"api_key_env" json:"api_key_env,omitempty"`
	Model         string   `yaml:"model" json:"model"`
	Capabilities  []string `yaml:"capabilities" json:"capabilities,omitempty"`
	ContextWindow int      `yaml:"context_window" json:"context_window,omitempty"`
}

type Runtime struct {
	Kind           string            `yaml:"kind" json:"kind"`
	Runner         string            `yaml:"runner" json:"runner,omitempty"`
	Workspace      string            `yaml:"workspace" json:"workspace,omitempty"`
	Invoke         RuntimeInvoke     `yaml:"invoke" json:"invoke,omitempty"`
	Env            map[string]string `yaml:"env" json:"env,omitempty"`
	EnvFrom        []string          `yaml:"env_from" json:"env_from,omitempty"`
	Capabilities   map[string]any    `yaml:"capabilities" json:"capabilities,omitempty"`
	Trust          string            `yaml:"trust" json:"trust,omitempty"`
	TimeoutSeconds int               `yaml:"timeout_seconds" json:"timeout_seconds,omitempty"`
}

type RuntimeInvoke struct {
	Executable string   `yaml:"executable" json:"executable,omitempty"`
	Args       []string `yaml:"args" json:"args,omitempty"`
	Stdin      string   `yaml:"stdin" json:"stdin,omitempty"`
}

type Agent struct {
	Kind         string   `yaml:"kind" json:"kind"`
	Model        string   `yaml:"model" json:"model,omitempty"`
	Runtime      string   `yaml:"runtime" json:"runtime,omitempty"`
	Endpoint     string   `yaml:"endpoint" json:"endpoint,omitempty"`
	Role         string   `yaml:"role" json:"role,omitempty"`
	Instructions string   `yaml:"instructions" json:"instructions,omitempty"`
	Tools        []string `yaml:"tools" json:"tools,omitempty"`
	Subagents    []string `yaml:"subagents" json:"subagents,omitempty"`
	Trust        string   `yaml:"trust" json:"trust,omitempty"`
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
