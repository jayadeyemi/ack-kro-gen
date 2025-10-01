package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type GraphEntry struct {
	Service string `yaml:"service"`
	Version string `yaml:"version"`
}

type Graphs struct {
	Graphs []GraphEntry `yaml:"graphs"`
}

func LoadGraphs(path string) (*Graphs, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var g Graphs
	if err := yaml.Unmarshal(b, &g); err != nil {
		return nil, err
	}
	return &g, nil
}
