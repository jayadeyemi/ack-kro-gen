package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

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
	if len(r.Graphs) == 0 {
		return nil, errors.New("graphs: at least one service is required")
	}
	for i := range r.Graphs {
		g := &r.Graphs[i]

		g.Service = strings.TrimSpace(g.Service)
		if g.Service == "" {
			return nil, fmt.Errorf("graphs[%d]: service is required", i)
		}

		g.Version = strings.TrimSpace(g.Version)
		if g.Version == "" {
			return nil, fmt.Errorf("graphs[%d]: version is required", i)
		}

		g.Image.Tag = strings.TrimSpace(g.Image.Tag)
		if g.Image.Tag == "" {
			g.Image.Tag = g.Version
		}

		g.ReleaseName = strings.TrimSpace(g.ReleaseName)
		if g.ReleaseName == "" {
			g.ReleaseName = fmt.Sprintf("ack-%s-controller", strings.ToLower(g.Service))
		}

		g.Namespace = strings.TrimSpace(g.Namespace)
		if g.Namespace == "" {
			g.Namespace = "ack-system"
		}
	}
	return &r, nil
}
