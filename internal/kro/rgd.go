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
	APIVersion string        `yaml:"apiVersion"`
	Kind       string        `yaml:"kind"`
	Metadata   Metadata      `yaml:"metadata"`
	Spec       RGDSpec       `yaml:"spec"`
}

type Metadata struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}

type RGDSpec struct {
	Schema    Schema                  `yaml:"schema"`
	Resources []Resource              `yaml:"resources"`
}

type Schema struct {
	APIVersion string      `yaml:"apiVersion"`
	Kind       string      `yaml:"kind"`
	Spec       SchemaSpec  `yaml:"spec"`
}

type SchemaSpec struct {
	Name        string                 `yaml:"name"`
	Namespace   string                 `yaml:"namespace"`
	Values      map[string]any         `yaml:"values"`
}

type Resource struct {
	ID       string                 `yaml:"id"`
	Template map[string]any         `yaml:"template"`
}

func EmitRGD(gs config.GraphSpec, r *render.Result, outPath string, outDir string) error {
	absOutDir, _ := filepath.Abs(outDir)
	absFile, _ := filepath.Abs(outPath)
	if !strings.HasPrefix(absFile, absOutDir+string(filepath.Separator)) {
		return errors.New("refusing to write outside the output directory")
	}

	// Build object list: CRDs first, then templates
	var objs []classify.Obj
	for _, crd := range r.CRDs {
		crd2, err := placeholders.ReplaceYAMLScalars(crd)
		if err != nil { return fmt.Errorf("placeholder on CRD: %w", err) }
		o, err := classify.Parse(crd2)
		if err != nil { return fmt.Errorf("parse CRD: %w", err) }
		objs = append(objs, o)
	}
	for _, body := range r.RenderedFiles {
		for _, doc := range util.SplitYAML(body) {
			if strings.TrimSpace(doc) == "" { continue }
			repl, err := placeholders.ReplaceYAMLScalars(doc)
			if err != nil { return fmt.Errorf("placeholder replace: %w", err) }
			o, err := classify.Parse(repl)
			if err != nil { return fmt.Errorf("parse manifest: %w", err) }
			objs = append(objs, o)
		}
	}

	g := classify.Classify(objs)

	ordered := append([]classify.Obj{}, g.CRDs...)
	ordered = append(ordered, g.Core...)
	ordered = append(ordered, g.RBAC...)
	ordered = append(ordered, g.Deployments...)
	ordered = append(ordered, g.Others...)

	resources := make([]Resource, 0, len(ordered))
	seen := map[string]struct{}{}
	for _, o := range ordered {
		var m map[string]any
		if err := yaml.Unmarshal([]byte(o.RawYAML), &m); err != nil { return err }
		id := makeID(o.Kind + "-" + o.Name)
		if _, ok := seen[id]; ok {
			for i := 2; ; i++ {
				alt := fmt.Sprintf("%s-%d", id, i)
				if _, ok := seen[alt]; !ok { id = alt; break }
			}
		}
		seen[id] = struct{}{}
		resources = append(resources, Resource{ID: id, Template: m})
	}

	serviceUpper := strings.ToUpper(gs.Service[:1]) + gs.Service[1:]

	s := Schema{
		APIVersion: "v1alpha1",
		Kind:       serviceUpper + "controller",
		Spec: SchemaSpec{
			Name:      "${schema.spec.name}",
			Namespace: "${schema.spec.namespace}",
			Values: map[string]any{
				"aws": map[string]any{
					"accountID":          "${schema.spec.values.aws.accountID}", // required
					"region":             "${schema.spec.values.aws.region}",
				},
				"deployment": map[string]any{
					"containerPort": 8080,
					"replicas":      1,
				},
				"iamRole": map[string]any{
					"oidcProvider":       "${schema.spec.values.iamRole.oidcProvider}", // required
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

	rgd := RGD{
		APIVersion: "kro.run/v1alpha1",
		Kind:       "ResourceGraphDefinition",
		Metadata: Metadata{
			Name:      fmt.Sprintf("ack-%s-ctrl.kro.run", gs.Service),
			Namespace: "kro",
		},
		Spec: RGDSpec{
			Schema:    s,
			Resources: resources,
		},
	}

	b, err := marshalYAML(rgd)
	if err != nil { return err }
	if err := os.WriteFile(outPath, b, 0o644); err != nil { return err }
	return nil
}

func marshalYAML(v any) ([]byte, error) {
	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil { return nil, err }
	_ = enc.Close()
	return []byte(buf.String()), nil
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9-]`)

func makeID(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" { s = "res" }
	return s
}