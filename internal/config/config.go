package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Root struct {
	Graphs []ValuesSpec `yaml:"graphs"`
}

type ValuesSpec struct {
	// Graph.yaml fields
	Service     string `yaml:"service"`
	Version     string `yaml:"version"`
	Namespace   string `yaml:"namespace"`
	ReleaseName string `yaml:"releaseName"`

	// Helm chart values fields
	Image            ImageSpec          `yaml:"image"`
	NameOverride     string             `yaml:"nameOverride"`
	FullnameOverride string             `yaml:"fullnameOverride"`
	Deployment       DeploymentSpec     `yaml:"deployment"`
	Role             RoleSpec           `yaml:"role"`
	Metrics          MetricsSpec        `yaml:"metrics"`
	Resources        ResourcesSpec      `yaml:"resources"`
	AWS              AWSSpec            `yaml:"aws"`
	Log              LogSpec            `yaml:"log"`
	InstallScope     string             `yaml:"installScope"`
	WatchNamespace   string             `yaml:"watchNamespace"`
	WatchSelectors   string             `yaml:"watchSelectors"`
	ResourceTags     []string           `yaml:"resourceTags"`
	DeletionPolicy   string             `yaml:"deletionPolicy"`
	Reconcile        ReconcileSpec      `yaml:"reconcile"`
	ServiceAccount   ServiceAccountSpec `yaml:"serviceAccount"`
	LeaderElection   LeaderElectionSpec `yaml:"leaderElection"`
	EnableCARM       bool               `yaml:"enableCARM"`
	FeatureGates     FeatureGatesSpec   `yaml:"featureGates"`
}

// image
type ImageSpec struct {
	Repository  string   `yaml:"repository"`
	Tag         string   `yaml:"tag"`
	PullPolicy  string   `yaml:"pullPolicy"`
	PullSecrets []string `yaml:"pullSecrets"`
}

// deployment
type DeploymentSpec struct {
	Annotations       map[string]string `yaml:"annotations"`
	Labels            map[string]string `yaml:"labels"`
	ContainerPort     int               `yaml:"containerPort"`
	Replicas          int               `yaml:"replicas"`
	NodeSelector      map[string]string `yaml:"nodeSelector"`
	Tolerations       []string          `yaml:"tolerations"`
	Affinity          map[string]any    `yaml:"affinity"`
	PriorityClassName string            `yaml:"priorityClassName"`
	HostNetwork       bool              `yaml:"hostNetwork"`
	DNSPolicy         string            `yaml:"dnsPolicy"`
	Strategy          map[string]any    `yaml:"strategy"`
	ExtraVolumes      []string          `yaml:"extraVolumes"`
	ExtraVolumeMounts []string          `yaml:"extraVolumeMounts"`
	ExtraEnvVars      []string          `yaml:"extraEnvVars"`
}

// role
type RoleSpec struct {
	Labels map[string]string `yaml:"labels"`
}

// metrics
type MetricsSpec struct {
	Service MetricsServiceSpec `yaml:"service"`
}

type MetricsServiceSpec struct {
	Create bool   `yaml:"create"`
	Type   string `yaml:"type"`
}

// resources
type ResourcesSpec struct {
	Requests map[string]string `yaml:"requests"`
	Limits   map[string]string `yaml:"limits"`
}

// aws
type AWSSpec struct {
	Region      string             `yaml:"region"`
	EndpointURL string             `yaml:"endpoint_url"`
	Credentials AWSCredentialsSpec `yaml:"credentials"`
}

type AWSCredentialsSpec struct {
	SecretName string `yaml:"secretName"`
	SecretKey  string `yaml:"secretKey"`
	Profile    string `yaml:"profile"`
}

// log
type LogSpec struct {
	EnableDevelopmentLogging bool   `yaml:"enable_development_logging"`
	Level                    string `yaml:"level"`
}

// reconcile
type ReconcileSpec struct {
	DefaultResyncPeriod        int            `yaml:"defaultResyncPeriod"`
	ResourceResyncPeriods      map[string]int `yaml:"resourceResyncPeriods"`
	DefaultMaxConcurrentSyncs  int            `yaml:"defaultMaxConcurrentSyncs"`
	ResourceMaxConcurrentSyncs map[string]int `yaml:"resourceMaxConcurrentSyncs"`
	Resources                  []string       `yaml:"resources"`
}

// serviceAccount
type ServiceAccountSpec struct {
	Create      bool              `yaml:"create"`
	Name        string            `yaml:"name"`
	Annotations map[string]string `yaml:"annotations"`
}

// leaderElection
type LeaderElectionSpec struct {
	Enabled   bool   `yaml:"enabled"`
	Namespace string `yaml:"namespace"`
}

// featureGates
type FeatureGatesSpec struct {
	ServiceLevelCARM  bool `yaml:"ServiceLevelCARM"`
	TeamLevelCARM     bool `yaml:"TeamLevelCARM"`
	ReadOnlyResources bool `yaml:"ReadOnlyResources"`
	ResourceAdoption  bool `yaml:"ResourceAdoption"`
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
	var rawRoot struct {
		Graphs []map[string]any `yaml:"graphs"`
	}
	if err := yaml.Unmarshal(b, &rawRoot); err != nil {
		return nil, err
	}

	getMap := func(m map[string]any, key string) (map[string]any, bool) {
		if m == nil {
			return nil, false
		}
		raw, ok := m[key]
		if !ok {
			return nil, false
		}
		switch typed := raw.(type) {
		case map[string]any:
			return typed, true
		case map[interface{}]any:
			converted := make(map[string]any, len(typed))
			for k, v := range typed {
				ks, ok := k.(string)
				if !ok {
					continue
				}
				converted[ks] = v
			}
			return converted, true
		default:
			return nil, false
		}
	}

	hasKey := func(m map[string]any, key string) bool {
		if m == nil {
			return false
		}
		_, ok := m[key]
		return ok
	}
	for i := range r.Graphs {
		g := &r.Graphs[i]
		var raw map[string]any
		if i < len(rawRoot.Graphs) {
			raw = rawRoot.Graphs[i]
		}

		g.Service = strings.TrimSpace(g.Service)
		if g.Service == "" {
			return nil, fmt.Errorf("graphs[%d]: service is required", i)
		}

		g.Version = strings.TrimSpace(g.Version)
		if g.Version == "" {
			return nil, fmt.Errorf("graphs[%d]: version is required", i)
		}

		g.Image.Repository = strings.TrimSpace(g.Image.Repository)
		if g.Image.Repository == "" {
			g.Image.Repository = fmt.Sprintf(
				"public.ecr.aws/aws-controllers-k8s/%s-controller",
				strings.ToLower(g.Service),
			)
		}

		if raw == nil {
			g.ServiceAccount.Create = true
			g.EnableCARM = true
			g.FeatureGates.ReadOnlyResources = true
			g.FeatureGates.ResourceAdoption = true
			continue
		}

		saMap, ok := getMap(raw, "serviceAccount")
		if !ok || !hasKey(saMap, "create") {
			g.ServiceAccount.Create = true
		}

		if !hasKey(raw, "enableCARM") {
			g.EnableCARM = true
		}

		fgMap, ok := getMap(raw, "featureGates")
		if !ok {
			g.FeatureGates.ReadOnlyResources = true
			g.FeatureGates.ResourceAdoption = true
			continue
		}
		if !hasKey(fgMap, "ReadOnlyResources") {
			g.FeatureGates.ReadOnlyResources = true
		}
		if !hasKey(fgMap, "ResourceAdoption") {
			g.FeatureGates.ResourceAdoption = true
		}

	}
	return &r, nil
}
