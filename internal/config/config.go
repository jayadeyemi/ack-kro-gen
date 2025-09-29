package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Root struct {
	Graphs []GraphSpec `yaml:"graphs"`
}

type GraphSpec struct {
	Service       string         `yaml:"service"`
	Version       string         `yaml:"version"`
	ReleaseName   string         `yaml:"releaseName"`
	Namespace     string         `yaml:"namespace"`
	Image         ImageSpec      `yaml:"image"`
	ServiceAccount SASpec        `yaml:"serviceAccount"`
	Controller    ControllerSpec `yaml:"controller"`
	Extras        ExtrasSpec     `yaml:"extras"`
}

type ImageSpec struct {
	Repository string `yaml:"repository"`
	Tag        string `yaml:"tag"`
}

type SASpec struct {
	Name        string            `yaml:"name"`
	Annotations map[string]string `yaml:"annotations"`
}

type ControllerSpec struct {
	LogLevel  string `yaml:"logLevel"`
	LogDev    string `yaml:"logDev"`
	AWSRegion string `yaml:"awsRegion"`
}

type ExtrasSpec struct {
	Values map[string]any `yaml:"values"`
}

func Load(path string) (*Root, error) {
	b, err := os.ReadFile(path)
	if err != nil { return nil, err }
	var r Root
	if err := yaml.Unmarshal(b, &r); err != nil { return nil, err }
	if len(r.Graphs) == 0 { return nil, errors.New("graphs: at least one service is required") }
	for i, g := range r.Graphs {
		if g.Service == "" || g.Version == "" { return nil, fmt.Errorf("graphs[%d]: service and version are required", i) }
		if g.ReleaseName == "" || g.Namespace == "" { return nil, fmt.Errorf("graphs[%d]: releaseName and namespace are required", i) }
	}
	return &r, nil
}