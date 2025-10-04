package kro

import (
	"fmt"
	"strings"

	"github.com/jayadeyemi/ack-kro-gen/internal/classify"
	"github.com/jayadeyemi/ack-kro-gen/internal/config"
	"github.com/jayadeyemi/ack-kro-gen/internal/placeholders"
	"gopkg.in/yaml.v3"
)

// Build controller-side resources from non-CRD objects.
func buildControllerResources(list []classify.Obj) ([]Resource, error) {
	res := make([]Resource, 0, len(list))
	seen := map[string]int{}
	for _, o := range list {
		var m map[string]any
		if err := yaml.Unmarshal([]byte(o.RawYAML), &m); err != nil {
			return nil, err
		}
		normalizeControllerResource(m)
		id := controllerIDForKind(o.Kind)
		seen[id]++
		if seen[id] > 1 {
			id = fmt.Sprintf("%s-%d", id, seen[id])
		}
		res = append(res, Resource{ID: id, Template: m})
	}
	return res, nil
}

func controllerIDForKind(kind string) string {
	k := strings.ToLower(strings.TrimSpace(kind))
	return "graph-" + makeID(k)
}

// MakeCtrlRGD assembles the controller RGD for a service.
func MakeCtrlRGD(gs config.ValuesSpec, serviceUpper string, ctrlResources []Resource, crdKinds []string) RGD {
	// Add a graph-crd item as the first resource in the controller graph.
	ctrlResources = append([]Resource{makeGraphCRDItem(gs.Service, serviceUpper)}, ctrlResources...)

	// Assemble the RGD object.
	return RGD{
		APIVersion: "kro.run/v1alpha1",
		Kind:       "ResourceGraphDefinition",
		Metadata: Metadata{
			Name:      fmt.Sprintf("ack-%s-ctrl.kro.run", gs.Service),
			Namespace: "kro",
		},
		Spec: RGDSpec{
			Schema:    CtrlSchema(gs, serviceUpper, crdKinds),
			Resources: ctrlResources,
		},
	}
}

// CtrlSchema assembles the schema for controller graphs using shared placeholders.

func CtrlSchema(gs config.ValuesSpec, serviceUpper string, crdKinds []string) Schema {
	values := placeholders.ControllerValues(gs, crdKinds)
	spec := buildSchemaSpec(gs, fmt.Sprintf("ack-%s-controller", gs.Service), values)
	return Schema{
		APIVersion: "v1alpha1",
		Kind:       serviceUpper + "controller",
		Spec:       spec,
	}
}

// define the graph-crd item to be added to the controller resources
func makeGraphCRDItem(service string, serviceUpper string) Resource {
	return Resource{
		ID: "graph-" + service + "-crds",
		Template: map[string]any{
			"apiVersion": "kro.run/v1alpha1",
			"kind":       serviceUpper + "crdgraph",
			"metadata": map[string]any{
				"name": "${schema.spec.name}-crd-graph",
			},
			"spec": map[string]any{
				"name": "${schema.spec.name}-crd-graph",
			},
		},
	}
}
