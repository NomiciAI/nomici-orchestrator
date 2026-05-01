package agentspec

import (
	"bytes"
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type LoadedSpec struct {
	Spec    *Spec
	Sources map[string]Source
	File    string
	Raw     []byte
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
