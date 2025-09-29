package kro

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/example/ack-kro-gen/internal/classify"
	"github.com/example/ack-kro-gen/internal/config"
	"github.com/example/ack-kro-gen/internal/placeholders"
	"github.com/example/ack-kro-gen/internal/render"
	"github.com/example/ack-kro-gen/internal/util"
	"gopkg.in/yaml.v3"
)

type RGD struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       RGDSpec  `yaml:"spec"`
}

type Metadata struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}

type RGDSpec struct {
	Schema    Schema     `yaml:"schema"`
	Resources []Resource `yaml:"resources"`
}

type Schema struct {
	APIVersion string     `yaml:"apiVersion"`
	Kind       string     `yaml:"kind"`
	Spec       SchemaSpec `yaml:"spec"`
}

type SchemaSpec struct {
	// For controller graphs we use Name, Namespace, Values
	// For CRD graphs only Name is relevant
	Name      string                 `yaml:"name"`
	Namespace string                 `yaml:"namespace,omitempty"`
	Values    map[string]any         `yaml:"values,omitempty"`
}

type Resource struct {
	ID       string                 `yaml:"id"`
	Template map[string]any         `yaml:"template"`
}

// EmitRGDs writes two RGDs per service:
// - ack/<service>-crds.yaml  (metadata.name = ack-<service>-crds.kro.run)
// - ack/<service>-ctrl.yaml  (metadata.name = ack-<service>-ctrl.kro.run)
func EmitRGDs(gs config.GraphSpec, r *render.Result, outDir string) ([]string, error) {
	absOutDir, _ := filepath.Abs(outDir)

	// Collect and classify all objects with placeholder substitution
	var objs []classify.Obj
	for _, crd := range r.CRDs {
		crd2, err := placeholders.ReplaceYAMLScalars(crd)
		if err != nil {
			return nil, fmt.Errorf("placeholder on CRD: %w", err)
		}
		o, err := classify.Parse(crd2)
		if err != nil {
			return nil, fmt.Errorf("parse CRD: %w", err)
		}
		objs = append(objs, o)
	}
	for _, body := range r.RenderedFiles {
		for _, doc := range util.SplitYAML(body) {
			if strings.TrimSpace(doc) == "" {
				continue
			}
			repl, err := placeholders.ReplaceYAMLScalars(doc)
			if err != nil {
				return nil, fmt.Errorf("placeholder replace: %w", err)
			}
			o, err := classify.Parse(repl)
			if err != nil {
				return nil, fmt.Errorf("parse manifest: %w", err)
			}
			objs = append(objs, o)
		}
	}

	groups := classify.Classify(objs)

	// Build resource lists
	crdResources := make([]Resource, 0, len(groups.CRDs))
	ctrlResources := []classify.Obj{}
	ctrlResources = append(ctrlResources, groups.Core...)
	ctrlResources = append(ctrlResources, groups.RBAC...)
	ctrlResources = append(ctrlResources, groups.Deployments...)
	ctrlResources = append(ctrlResources, groups.Others...)

	buildResources := func(list []classify.Obj) ([]Resource, error) {
		res := make([]Resource, 0, len(list))
		seen := map[string]struct{}{}
		for _, o := range list {
			var m map[string]any
			if err := yaml.Unmarshal([]byte(o.RawYAML), &m); err != nil {
				return nil, err
			}
			id := makeID(o.Kind + "-" + o.Name)
			if _, ok := seen[id]; ok {
				for i := 2; ; i++ {
					alt := fmt.Sprintf("%s-%d", id, i)
					if _, ok := seen[alt]; !ok {
						id = alt
						break
					}
				}
			}
			seen[id] = struct{}{}
			res = append(res, Resource{ID: id, Template: m})
		}
		return res, nil
	}

	var err error
	crdResources, err = buildResources(groups.CRDs)
	if err != nil {
		return nil, err
	}
	ctrlResourcesBuilt, err := buildResources(ctrlResources)
	if err != nil {
		return nil, err
	}

	// Controller schema with values
	serviceUpper := strings.ToUpper(gs.Service[:1]) + gs.Service[1:]
	ctrlSchema := Schema{
		APIVersion: "v1alpha1",
		Kind:       serviceUpper + "controller",
		Spec: SchemaSpec{
			Name:      "${schema.spec.name}",
			Namespace: "${schema.spec.namespace}",
			Values: map[string]any{
				"aws": map[string]any{
					"accountID": "${schema.spec.values.aws.accountID}",
					"region":    "${schema.spec.values.aws.region}",
				},
				"deployment": map[string]any{
					"containerPort": 8080,
					"replicas":      1,
				},
				"iamRole": map[string]any{
					"oidcProvider":       "${schema.spec.values.iamRole.oidcProvider}",
					"maxSessionDuration": 3600,
				},
				"image": map[string]any{
					"repository":  "${schema.spec.values.image.repository}",
					"tag":         "${schema.spec.values.image.tag}",
					"deletePolicy": "${schema.spec.values.image.deletePolicy}",
					"resources": map[string]any{
						"requests": map[string]any{"memory": "64Mi", "cpu": "50m"},
						"limits":   map[string]any{"memory": "128Mi", "cpu": "100m"},
					},
				},
				"log": map[string]any{
					"enabled": "${schema.spec.values.log.enabled}",
					"level":   "${schema.spec.values.log.level}",
				},
				"serviceAccount": map[string]any{
					"name": "${schema.spec.values.serviceAccount.name}",
				},
			},
		},
	}

	// CRD schema minimal
	crdSchema := Schema{
		APIVersion: "v1alpha1",
		Kind:       serviceUpper + "crdgraph",
		Spec: SchemaSpec{
			Name: "${schema.spec.name}",
		},
	}

	// Compose RGDs
	crdsRGD := RGD{
		APIVersion: "kro.run/v1alpha1",
		Kind:       "ResourceGraphDefinition",
		Metadata: Metadata{
			Name:      fmt.Sprintf("ack-%s-crds.kro.run", gs.Service),
			Namespace: "kro",
		},
		Spec: RGDSpec{
			Schema:    crdSchema,
			Resources: crdResources,
		},
	}
	ctrlRGD := RGD{
		APIVersion: "kro.run/v1alpha1",
		Kind:       "ResourceGraphDefinition",
		Metadata: Metadata{
			Name:      fmt.Sprintf("ack-%s-ctrl.kro.run", gs.Service),
			Namespace: "kro",
		},
		Spec: RGDSpec{
			Schema:    ctrlSchema,
			Resources: ctrlResourcesBuilt,
		},
	}

	// File paths derived from metadata.name, placed under ack/<service>-*.yaml
	outAckDir := filepath.Join(absOutDir, "ack")
	if err := os.MkdirAll(outAckDir, 0o755); err != nil {
		return nil, err
	}
	crdsPath := filepath.Join(outAckDir, fmt.Sprintf("%s-crds.yaml", gs.Service))
	ctrlPath := filepath.Join(outAckDir, fmt.Sprintf("%s-ctrl.yaml", gs.Service))

	// Safety: keep writes inside outDir
	for _, p := range []string{crdsPath, ctrlPath} {
		absP, _ := filepath.Abs(p)
		if !strings.HasPrefix(absP, absOutDir+string(filepath.Separator)) {
			return nil, errors.New("refusing to write outside the output directory")
		}
	}

	if err := writeYAML(crdsPath, crdsRGD); err != nil {
		return nil, err
	}
	if err := writeYAML(ctrlPath, ctrlRGD); err != nil {
		return nil, err
	}
	return []string{crdsPath, ctrlPath}, nil
}

func writeYAML(path string, v any) error {
	b, err := marshalYAML(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func marshalYAML(v any) ([]byte, error) {
	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	_ = enc.Close()
	return []byte(buf.String()), nil
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9-]`)

func makeID(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "res"
	}
	return s
}
