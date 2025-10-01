package kro

import (
	"fmt"
	"strings"

	"github.com/jayadeyemi/ack-kro-gen/internal/classify"
	"github.com/jayadeyemi/ack-kro-gen/internal/config"
	"gopkg.in/yaml.v3"
)

// Build CRD resources from CRD objects.
func buildCRDResources(list []classify.Obj) ([]Resource, error) {
	res := make([]Resource, 0, len(list))
	seen := map[string]int{}
	for _, o := range list {
		var m map[string]any
		if err := yaml.Unmarshal([]byte(o.RawYAML), &m); err != nil {
			return nil, err
		}
		base := o.Name
		if idx := strings.Index(base, "."); idx > 0 {
			base = base[:idx]
		}
		id := "graph-" + makeID(base)
		seen[id]++
		if seen[id] > 1 {
			id = fmt.Sprintf("%s-%d", id, seen[id])
		}
		res = append(res, Resource{ID: id, Template: m})
	}
	return res, nil
}

// MakeCRDsRGD assembles the CRDs RGD for a service.
func MakeCRDsRGD(gs config.GraphSpec, serviceUpper string, crdResources []Resource) RGD {
	return RGD{
		APIVersion: "kro.run/v1alpha1",
		Kind:       "ResourceGraphDefinition",
		Metadata: Metadata{
			Name:      fmt.Sprintf("ack-%s-crds.kro.run", gs.Service),
			Namespace: "kro",
		},
		Spec: RGDSpec{
			Schema:    newCRDSchema(serviceUpper),
			Resources: crdResources,
		},
	}
}

func newCRDSchema(serviceUpper string) Schema {
	return Schema{
		APIVersion: "v1alpha1",
		Kind:       serviceUpper + "crdgraph",
		Spec: SchemaSpec{
			Name: "${schema.spec.name}",
		},
	}
}
