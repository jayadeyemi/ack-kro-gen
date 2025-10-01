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
	Service        string         `yaml:"service"`
	Version        string         `yaml:"version"`
	ReleaseName    string         `yaml:"releaseName"`
	Namespace      string         `yaml:"namespace"`
	AWS            AWSSpec        `yaml:"aws"`
	Image          ImageSpec      `yaml:"image"`
	ServiceAccount SASpec         `yaml:"serviceAccount"`
	Controller     ControllerSpec `yaml:"controller"`
	Extras         ExtrasSpec     `yaml:"extras"`
}

type ImageSpec struct {
	Repository string `yaml:"repository"`
	Tag        string `yaml:"tag"`
}

type SASpec struct {
	Name        string            `yaml:"name"`
	Annotations map[string]string `yaml:"annotations"`
}

type AWSSpec struct {
	Region      string `yaml:"region"`
	AccountID   string `yaml:"accountID"`
	Credentials string `yaml:"credentials"`
	SecretName  string `yaml:"secretName"`
	Profile     string `yaml:"profile"`
}
type ControllerSpec struct {
	LogLevel       string `yaml:"logLevel"`
	LogDev         string `yaml:"logDev"`
	WatchNamespace string `yaml:"watchNamespace"`
}

type ExtrasSpec struct {
	Values map[string]any `yaml:"values"`
}

func Load(path string) (*Root, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Root
	if err := yaml.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	for i := range r.Graphs {
		applyGraphDefaults(&r.Graphs[i])
	}
	if len(r.Graphs) == 0 {
		return nil, errors.New("graphs: at least one service is required")
	}
	for i := range r.Graphs {
		g := &r.Graphs[i]
		if g.Service == "" {
			return nil, fmt.Errorf("graphs[%d]: service is required", i)
		}
		if g.Version == "" {
			return nil, fmt.Errorf("graphs[%d]: version is required", i)
		}
		if g.Image.Tag == "" {
			return nil, fmt.Errorf("graphs[%d]: image tag is required", i)
		}
		if g.ReleaseName == "" {
			return nil, fmt.Errorf("graphs[%d]: releaseName is required", i)
		}
		if g.Namespace == "" {
			return nil, fmt.Errorf("graphs[%d]: namespace is required", i)
		}
	}
	return &r, nil
}
