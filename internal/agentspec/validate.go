package agentspec

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
)

const (
	AgentKindGateway  = "gateway_agent"
	AgentKindExternal = "external_agent"
	AgentKindModel    = "model_agent"
	AgentKindTool     = "tool_agent"

	RuntimeKindCLIAgent = "cli_agent"
)

func Validate(loaded *LoadedSpec) []ValidationError {
	if loaded == nil || loaded.Spec == nil {
		return []ValidationError{{
			Code:        "invalid_spec",
			Message:     "AgentSpec is empty.",
			Remediation: "Create a nomici.yaml with version and project fields.",
			Source:      Source{Path: "version"},
		}}
	}

	spec := loaded.Spec
	var errors []ValidationError

	if strings.TrimSpace(spec.Version) == "" {
		errors = append(errors, validationError(loaded, "missing_required", "version", "version is required", "Set version: \"0.1\"."))
	} else if spec.Version != "0.1" {
		errors = append(errors, validationError(loaded, "unsupported_version", "version", "only AgentSpec version 0.1 is supported", "Set version: \"0.1\"."))
	}
	if strings.TrimSpace(spec.Project.Name) == "" {
		errors = append(errors, validationError(loaded, "missing_required", "project.name", "project.name is required", "Set project.name to a stable project identifier."))
	}

	for id, model := range spec.Models {
		path := "models." + id
		if strings.TrimSpace(model.Kind) == "" {
			errors = append(errors, validationError(loaded, "missing_required", path+".kind", "model kind is required", "Set kind to openai_compatible or ollama."))
		} else if !providers.KnownKind(model.Kind) {
			errors = append(errors, validationError(loaded, "unsupported_model_kind", path+".kind", "model kind "+quote(model.Kind)+" is not implemented in v0.1", "Use openai_compatible or ollama for the current proof slice."))
		}
		if strings.TrimSpace(model.Model) == "" {
			errors = append(errors, validationError(loaded, "missing_required", path+".model", "model name is required", "Set model to the provider model name."))
		}
		if providers.NormalizeKind(model.Kind) == providers.KindOpenAICompatible && strings.TrimSpace(model.APIKeyEnv) == "" {
			errors = append(errors, validationError(loaded, "missing_required", path+".api_key_env", "api_key_env is required for openai_compatible models", "Set api_key_env to an environment variable name such as OPENAI_API_KEY."))
		}
	}

	for id, runtime := range spec.Runtimes {
		path := "runtimes." + id
		switch runtime.Kind {
		case RuntimeKindCLIAgent:
			if runtime.Invoke.Executable == "" {
				errors = append(errors, validationError(loaded, "missing_required", path+".invoke.executable", "cli_agent runtime requires invoke.executable", "Set invoke.executable to the command to run."))
			}
		case "":
			errors = append(errors, validationError(loaded, "missing_required", path+".kind", "runtime kind is required", "Set kind to cli_agent for the current proof slice."))
		default:
			errors = append(errors, validationError(loaded, "unsupported_runtime_kind", path+".kind", "runtime kind "+quote(runtime.Kind)+" is not executable in v0.1 yet", "Use cli_agent for Gate 4 external agent execution."))
		}
	}

	for id, agent := range spec.Agents {
		path := "agents." + id
		switch agent.Kind {
		case AgentKindGateway, AgentKindExternal, AgentKindModel, AgentKindTool:
		case "native_agent":
			errors = append(errors, validationError(loaded, "unsupported_agent_kind", path+".kind", "native_agent is not a v0.1 public kind", "Use gateway_agent for the minimal Gateway-run coordinator."))
		case "":
			errors = append(errors, validationError(loaded, "missing_required", path+".kind", "agent kind is required", "Set kind to gateway_agent, external_agent, model_agent, or tool_agent."))
		default:
			errors = append(errors, validationError(loaded, "unsupported_agent_kind", path+".kind", "agent kind "+quote(agent.Kind)+" is not supported", "Use gateway_agent, external_agent, model_agent, or tool_agent."))
		}

		if (agent.Kind == AgentKindGateway || agent.Kind == AgentKindModel) && strings.TrimSpace(agent.Model) == "" {
			errors = append(errors, validationError(loaded, "missing_required", path+".model", "model-backed agents require a model reference", "Set model to a key under models."))
		}
		if agent.Model != "" {
			if _, ok := spec.Models[agent.Model]; !ok {
				errors = append(errors, validationError(loaded, "missing_reference", path+".model", "agent references missing model "+quote(agent.Model), "Define models."+agent.Model+" or update the agent model reference."))
			}
		}
		if agent.Kind == AgentKindExternal {
			if strings.TrimSpace(agent.Runtime) == "" {
				errors = append(errors, validationError(loaded, "missing_required", path+".runtime", "external_agent requires a runtime reference", "Set runtime to a key under runtimes."))
			} else if _, ok := spec.Runtimes[agent.Runtime]; !ok {
				errors = append(errors, validationError(loaded, "missing_reference", path+".runtime", "agent references missing runtime "+quote(agent.Runtime), "Define runtimes."+agent.Runtime+" or update the runtime reference."))
			}
		}
	}

	for index, edge := range spec.Edges {
		path := "edges[" + itoa(index) + "]"
		if edge.From == "" {
			errors = append(errors, validationError(loaded, "missing_required", path+".from", "edge.from is required", "Set edge.from to an agent id."))
		} else if _, ok := spec.Agents[edge.From]; !ok {
			errors = append(errors, validationError(loaded, "missing_reference", path+".from", "edge.from references missing agent "+quote(edge.From), "Define agents."+edge.From+" or update the edge."))
		}
		if edge.To == "" {
			errors = append(errors, validationError(loaded, "missing_required", path+".to", "edge.to is required", "Set edge.to to an agent id."))
		} else if _, ok := spec.Agents[edge.To]; !ok {
			errors = append(errors, validationError(loaded, "missing_reference", path+".to", "edge.to references missing agent "+quote(edge.To), "Define agents."+edge.To+" or update the edge."))
		}
		if !knownEdgeMode(edge.Mode) {
			errors = append(errors, validationError(loaded, "unsupported_edge_mode", path+".mode", "edge mode "+quote(edge.Mode)+" is not supported", "Use handoff, a2a, tool_call, mcp, parallel, fallback, approval_required, memory_read, or memory_write."))
		}
	}

	return errors
}

func SourceHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func validationError(loaded *LoadedSpec, code string, path string, message string, remediation string) ValidationError {
	return ValidationError{
		Code:        code,
		Message:     message,
		Remediation: remediation,
		Source:      loaded.Source(path),
	}
}

func knownEdgeMode(mode string) bool {
	switch mode {
	case "handoff", "a2a", "tool_call", "mcp", "parallel", "fallback", "approval_required", "memory_read", "memory_write":
		return true
	default:
		return false
	}
}

func quote(value string) string {
	return `"` + value + `"`
}
