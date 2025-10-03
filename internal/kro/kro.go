package kro

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jayadeyemi/ack-kro-gen/internal/classify"
	"github.com/jayadeyemi/ack-kro-gen/internal/config"
	"github.com/jayadeyemi/ack-kro-gen/internal/placeholders"
	"github.com/jayadeyemi/ack-kro-gen/internal/render"
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
	Name      string         `yaml:"name"`
	Namespace string         `yaml:"namespace,omitempty"`
	Values    map[string]any `yaml:"values,omitempty"`
}

type Resource struct {
	ID       string         `yaml:"id"`
	Template map[string]any `yaml:"template"`
}

// EmitRGDs orchestrates parse → classify → build → write.
func EmitRGDs(gs config.GraphSpec, r *render.Result, outDir string) ([]string, error) {
	absOutDir, err := filepath.Abs(outDir)
	if err != nil {
		return nil, fmt.Errorf("resolve output dir: %w", err)
	}
	serviceUpper := toUpperService(gs.Service)

	var objs []classify.Obj
	for _, crd := range r.CRDs {

		o, err := classify.Parse(crd)
		if err != nil {
			return nil, fmt.Errorf("parse CRD: %w", err)
		}
		objs = append(objs, o)
	}
	for _, body := range r.RenderedFiles {
		for _, doc := range render.SplitYAML(body) {
			if strings.TrimSpace(doc) == "" {
				continue
			}

			o, err := classify.Parse(doc)
			if err != nil {
				return nil, fmt.Errorf("parse manifest: %w", err)
			}
			objs = append(objs, o)
		}
	}

	groups := classify.Classify(objs)

	// Build per-domain resources.
	crdResources, err := buildCRDResources(groups.CRDs)
	if err != nil {
		return nil, err
	}
	crdKinds := extractCRDKinds(groups.CRDs)
	ctrlResources, err := buildControllerResources(append(append(append(groups.Core, groups.RBAC...), groups.Deployments...), groups.Others...))
	if err != nil {
		return nil, err
	}

	// Build per-domain RGDs.
	crdsRGD := MakeCRDsRGD(gs, serviceUpper, crdResources, crdKinds)
	ctrlRGD := MakeCtrlRGD(gs, serviceUpper, ctrlResources, crdKinds)

	// Write files.
	outAckDir := filepath.Join(absOutDir, "ack")
	if err := os.MkdirAll(outAckDir, 0o755); err != nil {
		return nil, err
	}
	crdsPath := filepath.Join(outAckDir, fmt.Sprintf("%s-crds.yaml", gs.Service))
	ctrlPath := filepath.Join(outAckDir, fmt.Sprintf("%s-ctrl.yaml", gs.Service))

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

func extractCRDKinds(list []classify.Obj) []string {
	if len(list) == 0 {
		return nil
	}
	out := make([]string, 0, len(list))
	seen := map[string]struct{}{}
	for _, o := range list {
		var crd struct {
			Spec struct {
				Names struct {
					Kind string `yaml:"kind"`
				} `yaml:"names"`
			} `yaml:"spec"`
		}
		if err := yaml.Unmarshal([]byte(o.RawYAML), &crd); err != nil {
			continue
		}
		kind := strings.TrimSpace(crd.Spec.Names.Kind)
		if kind == "" {
			continue
		}
		if _, ok := seen[kind]; ok {
			continue
		}
		seen[kind] = struct{}{}
		out = append(out, kind)
	}
	return out
}

func writeYAML(path string, v any) error {
	b, err := marshalYAML(v)
	if err != nil {
		return err
	}
	// run legacy scalar replacer now
	out, err := placeholders.ReplaceYAMLScalars(string(b))
	if err != nil {
		return fmt.Errorf("placeholder replace: %w", err)
	}

	return os.WriteFile(path, []byte(out), 0o644)
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

func toUpperService(svc string) string {
	svc = strings.TrimSpace(svc)
	if svc == "" {
		return ""
	}
	if len(svc) == 1 {
		return strings.ToUpper(svc)
	}
	return strings.ToUpper(svc[:1]) + svc[1:]
}
