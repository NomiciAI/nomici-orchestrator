package agentspec

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

type LoadedSpec struct {
	Spec      *Spec
	Sources   map[string]Source
	File      string
	LocalFile string
	Raw       []byte
}

func LoadFile(path string) (*LoadedSpec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read AgentSpec: %w", err)
	}

	var spec Spec
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&spec); err != nil {
		return nil, fmt.Errorf("parse AgentSpec: %w", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("parse AgentSpec source map: %w", err)
	}

	sources := map[string]Source{}
	if len(root.Content) > 0 {
		walkSources(root.Content[0], path, "", sources)
	}

	return &LoadedSpec{
		Spec:    &spec,
		Sources: sources,
		File:    path,
		Raw:     raw,
	}, nil
}

func LoadFileWithLocal(path string) (*LoadedSpec, error) {
	loaded, err := LoadFile(path)
	if err != nil {
		return nil, err
	}
	localPath := LocalOverridePath(path)
	raw, err := os.ReadFile(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return loaded, nil
		}
		return nil, fmt.Errorf("read local AgentSpec override: %w", err)
	}
	var local Spec
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&local); err != nil {
		return nil, fmt.Errorf("parse local AgentSpec override: %w", err)
	}
	mergeSpec(loaded.Spec, &local)
	loaded.LocalFile = localPath
	loaded.Raw = append(append([]byte{}, loaded.Raw...), raw...)
	return loaded, nil
}

func LocalOverridePath(path string) string {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return "nomici.local.yaml"
	}
	return filepath.Join(dir, "nomici.local.yaml")
}

func (loaded *LoadedSpec) Source(path string) Source {
	if source, ok := loaded.Sources[path]; ok {
		return source
	}
	return Source{File: loaded.File, Path: path}
}

func walkSources(node *yaml.Node, file string, path string, sources map[string]Source) {
	if node == nil {
		return
	}
	if path != "" {
		sources[path] = Source{File: file, Path: path, Line: node.Line}
	}

	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			childPath := key.Value
			if path != "" {
				childPath = path + "." + key.Value
			}
			sources[childPath] = Source{File: file, Path: childPath, Line: value.Line}
			walkSources(value, file, childPath, sources)
		}
	case yaml.SequenceNode:
		for index, value := range node.Content {
			childPath := path + "[" + strconv.Itoa(index) + "]"
			sources[childPath] = Source{File: file, Path: childPath, Line: value.Line}
			walkSources(value, file, childPath, sources)
		}
	}
}

func mergeSpec(base *Spec, local *Spec) {
	if local == nil {
		return
	}
	if base.Models == nil && len(local.Models) > 0 {
		base.Models = map[string]Model{}
	}
	for id, model := range local.Models {
		base.Models[id] = model
	}
	if base.Runtimes == nil && len(local.Runtimes) > 0 {
		base.Runtimes = map[string]Runtime{}
	}
	for id, runtime := range local.Runtimes {
		base.Runtimes[id] = runtime
	}
	if base.Tools == nil && len(local.Tools) > 0 {
		base.Tools = map[string]map[string]any{}
	}
	for id, tool := range local.Tools {
		base.Tools[id] = tool
	}
	if base.Deployment == nil && len(local.Deployment) > 0 {
		base.Deployment = map[string]any{}
	}
	for key, value := range local.Deployment {
		base.Deployment[key] = value
	}
	if base.Profiles == nil && len(local.Profiles) > 0 {
		base.Profiles = map[string]any{}
	}
	for key, value := range local.Profiles {
		base.Profiles[key] = value
	}
	if base.Extensions == nil && len(local.Extensions) > 0 {
		base.Extensions = map[string]any{}
	}
	for key, value := range local.Extensions {
		base.Extensions[key] = value
	}
}
