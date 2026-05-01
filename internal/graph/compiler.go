package graph

import (
	"fmt"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/agentspec"
	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
	"github.com/NomiciAI/nomici-orchestrator/internal/providers"
)

func Compile(loaded *agentspec.LoadedSpec) (*Snapshot, []agentspec.ValidationError) {
	if errors := agentspec.Validate(loaded); len(errors) > 0 {
		return nil, errors
	}

	spec := loaded.Spec
	snapshot := &Snapshot{
		SnapshotID:    ids.New("graph"),
		SchemaVersion: spec.Version,
		ProjectID:     spec.Project.Name,
		CreatedAt:     time.Now().UTC(),
		SourceHash:    agentspec.SourceHash(loaded.Raw),
		IR: IR{
			Models:   map[string]Model{},
			Runtimes: map[string]Runtime{},
			Agents:   map[string]Agent{},
			Edges:    []Edge{},
		},
	}

	for id, model := range spec.Models {
		baseURL := model.BaseURL
		if baseURL == "" {
			baseURL = providers.DefaultBaseURL(model.Kind)
		}
		snapshot.IR.Models[id] = Model{
			ID:            id,
			Kind:          providers.NormalizeKind(model.Kind),
			BaseURL:       baseURL,
			APIKeyEnv:     model.APIKeyEnv,
			Model:         model.Model,
			Capabilities:  model.Capabilities,
			ContextWindow: model.ContextWindow,
			Source:        loaded.Source("models." + id),
		}
	}

	for id, runtime := range spec.Runtimes {
		snapshot.IR.Runtimes[id] = Runtime{
			ID:        id,
			Kind:      runtime.Kind,
			Runner:    runtime.Runner,
			Workspace: runtime.Workspace,
			Start: RuntimeStart{
				Command:    runtime.Start.Command,
				Executable: runtime.Start.Executable,
				Args:       runtime.Start.Args,
			},
			Invoke: RuntimeInvoke{
				Executable: runtime.Invoke.Executable,
				Args:       runtime.Invoke.Args,
				Stdin:      runtime.Invoke.Stdin,
			},
			Env:            runtime.Env,
			EnvFrom:        runtime.EnvFrom,
			Capabilities:   runtime.Capabilities,
			Trust:          runtime.Trust,
			TimeoutSeconds: runtime.TimeoutSeconds,
			Source:         loaded.Source("runtimes." + id),
		}
	}

	for id, agent := range spec.Agents {
		snapshot.IR.Agents[id] = Agent{
			ID:           id,
			Kind:         agent.Kind,
			Model:        agent.Model,
			Runtime:      agent.Runtime,
			Role:         agent.Role,
			Instructions: agent.Instructions,
			Source:       loaded.Source("agents." + id),
		}
	}

	for index, edge := range spec.Edges {
		snapshot.IR.Edges = append(snapshot.IR.Edges, Edge{
			ID:     fmt.Sprintf("edge_%d", index+1),
			From:   edge.From,
			To:     edge.To,
			Mode:   edge.Mode,
			Source: loaded.Source("edges[" + agentspecIndex(index) + "]"),
		})
	}

	return snapshot, nil
}

func agentspecIndex(index int) string {
	if index == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for index > 0 {
		i--
		buf[i] = byte('0' + index%10)
		index /= 10
	}
	return string(buf[i:])
}
