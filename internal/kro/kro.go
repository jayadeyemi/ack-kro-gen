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
	"github.com/jayadeyemi/ack-kro-gen/internal/util"
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
	absOutDir, _ := filepath.Abs(outDir)
	serviceUpper := strings.ToUpper(gs.Service[:1]) + gs.Service[1:]

	var objs []classify.Obj
	for _, crd := range r.CRDs {

		o, err := classify.Parse(crd)
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
	ctrlResources, err := buildControllerResources(append(append(append(groups.Core, groups.RBAC...), groups.Deployments...), groups.Others...))
	if err != nil {
		return nil, err
	}

	// Build per-domain RGDs.
	crdsRGD := MakeCRDsRGD(gs, serviceUpper, crdResources)
	ctrlRGD := MakeCtrlRGD(gs, serviceUpper, ctrlResources)
  
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
// Before writing YAML resources:
rendered := replace.ReplaceAll(templateText, false) // substitute sentinels -> ${schema...}

// When generating schema defaults/example:
defaultsDoc := replace.ReplaceAll(schemaTemplateText, true) // also substitutes defaults

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

// Generate renders all controllers and CRDs per graph spec.
func Generate(ctx context.Context, graphs config.GraphSpec, outDir, cacheDir string, offline bool, concurrency int, logLevel string) error {
    // 1) Render charts
    resEC2, err := render.RenderChart(ctx, cacheDir, offline, graphs.EC2)   // example selector
    if err != nil { return fmt.Errorf("render ec2: %w", err) }
    // ... render others per graphs ...

    // 2) Split YAML -> classify
    var objs []classify.Obj
    for _, s := range resEC2.RenderedFiles {
        for _, doc := range render.SplitYAML(s) {
            if strings.TrimSpace(doc) == "" { continue }
            o, err := classify.Parse(doc)
            if err != nil { return fmt.Errorf("parse manifest: %w", err) }
            objs = append(objs, o)
        }
    }
    groups := classify.Classify(objs)

    // 3) Build resources
    crdResources, err := buildCRDResources(groups.CRDs)
    if err != nil { return err }
    ctrlResources, err := buildControllerResources(append(append(append(groups.RBAC, groups.Deployments...), groups.Services...), groups.Other...) )
    if err != nil { return err }

    // 4) Assemble RGD and write
    rgd := RGD{
      APIVersion: "kro.run/v1alpha1",
      Kind: "ResourceGraphDefinition",
      Metadata: Metadata{Name: graphs.Name, Namespace: graphs.Namespace, Labels: graphs.Labels},
      Spec: RGDSpec{ Schema: Schema{APIVersion: "v1"}, Resources: append(crdResources, ctrlResources...) },
    }
    return writeRGD(outDir, graphs.Name+"-ctrl.yaml", rgd)
}

func writeRGD(dir, name string, v any) error {
  if err := os.MkdirAll(dir, 0o755); err != nil { return err }
  b, err := yaml.Marshal(v); if err != nil { return err }
  path := filepath.Join(dir, name)
  return os.WriteFile(path, b, 0o644)
}
