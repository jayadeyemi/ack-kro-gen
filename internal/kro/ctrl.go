package kro

import (
	"fmt"
	"strings"

	"github.com/jayadeyemi/ack-kro-gen/internal/classify"
	"github.com/jayadeyemi/ack-kro-gen/internal/config"
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
func MakeCtrlRGD(gs config.GraphSpec, serviceUpper string, ctrlResources []Resource) RGD {

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
			Schema:    newCtrlSchema(serviceUpper),
			Resources: ctrlResources,
		},
	}
}

// define the controller schema 
func newCtrlSchema(serviceUpper string) Schema {
	return Schema{
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




