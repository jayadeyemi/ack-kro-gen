package kro

import (
	"fmt"
	"strings"

	"github.com/jayadeyemi/ack-kro-gen/internal/classify"
	"github.com/jayadeyemi/ack-kro-gen/internal/config"
	"github.com/jayadeyemi/ack-kro-gen/internal/placeholders"
	"gopkg.in/yaml.v3"
)

// buildCRDResourceMapping maps CRD full names to resource types
func buildCRDResourceMapping() map[string]string {
	return map[string]string{
		// Common ACK resources
		"adoptedresources.services.k8s.aws": "AdoptedResource",
		"fieldexports.services.k8s.aws":     "FieldExport",
		// S3
		"buckets.s3.services.k8s.aws": "Bucket",
		// EC2
		"capacityreservations.ec2.services.k8s.aws":  "CapacityReservation",
		"dhcpoptions.ec2.services.k8s.aws":           "DHCPOptions",
		"elasticipaddresses.ec2.services.k8s.aws":    "ElasticIPAddress",
		"flowlogs.ec2.services.k8s.aws":              "FlowLog",
		"instances.ec2.services.k8s.aws":             "Instance",
		"internetgateways.ec2.services.k8s.aws":      "InternetGateway",
		"launchtemplates.ec2.services.k8s.aws":       "LaunchTemplate",
		"natgateways.ec2.services.k8s.aws":           "NATGateway",
		"networkacls.ec2.services.k8s.aws":           "NetworkACL",
		"routetables.ec2.services.k8s.aws":           "RouteTable",
		"securitygroups.ec2.services.k8s.aws":        "SecurityGroup",
		"subnets.ec2.services.k8s.aws":               "Subnet",
		"transitgateways.ec2.services.k8s.aws":       "TransitGateway",
		"vpcendpoints.ec2.services.k8s.aws":          "VPCEndpoint",
		"vpcpeeringconnections.ec2.services.k8s.aws": "VPCPeeringConnection",
		"vpcs.ec2.services.k8s.aws":                  "VPC",
		// RDS
		"dbclusters.rds.services.k8s.aws":               "DBCluster",
		"dbclusterendpoints.rds.services.k8s.aws":       "DBClusterEndpoint",
		"dbclusterparametergroups.rds.services.k8s.aws": "DBClusterParameterGroup",
		"dbclustersnapshots.rds.services.k8s.aws":       "DBClusterSnapshot",
		"dbinstances.rds.services.k8s.aws":              "DBInstance",
		"dbparametergroups.rds.services.k8s.aws":        "DBParameterGroup",
		"dbproxies.rds.services.k8s.aws":                "DBProxy",
		"dbsnapshots.rds.services.k8s.aws":              "DBSnapshot",
		"dbsubnetgroups.rds.services.k8s.aws":           "DBSubnetGroup",
		"globalclusters.rds.services.k8s.aws":           "GlobalCluster",
	}
}

// extractCRDFullName extracts CRD full name from metadata
func extractCRDFullName(crdObj map[string]any) string {
	if metadata, ok := crdObj["metadata"].(map[string]any); ok {
		if name, ok := metadata["name"].(string); ok {
			return name
		}
	}
	return ""
}

// Build CRD resources from CRD objects.
func buildCRDResources(list []classify.Obj, reconcileResources []string) ([]Resource, error) {
	res := make([]Resource, 0, len(list))
	seen := map[string]int{}
	crdResourceMap := buildCRDResourceMapping()

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

		// Extract CRD full name and map to resource type
		crdFullName := extractCRDFullName(m)
		resourceType := crdResourceMap[crdFullName]

		// Build includeWhen condition if resource type is found
		var includeWhen []string
		if resourceType != "" {
			// CEL: Check if resourceType is in schema.spec.values.reconcile.resources array
			celExpr := fmt.Sprintf(`${%q in schema.spec.values.reconcile.resources}`, resourceType)
			includeWhen = []string{celExpr}
		}

		res = append(res, Resource{
			ID:          id,
			IncludeWhen: includeWhen,
			Template:    m,
		})
	}
	return res, nil
}

// MakeCRDsRGD assembles the CRDs RGD for a service.
func MakeCRDsRGD(gs config.ValuesSpec, serviceUpper string, crdResources []Resource, crdKinds []string) RGD {
	return RGD{
		APIVersion: "kro.run/v1alpha1",
		Kind:       "ResourceGraphDefinition",
		Metadata: Metadata{
			Name:      fmt.Sprintf("ack-%s-crds.kro.run", gs.Service),
			Namespace: "kro",
		},
		Spec: RGDSpec{
			Schema:    CRDSchema(gs, serviceUpper, crdKinds),
			Resources: crdResources,
		},
	}
}

// CRDSchema assembles the schema for CRD graphs using shared placeholder handling.
func CRDSchema(gs config.ValuesSpec, serviceUpper string, crdKinds []string) Schema {
	values := map[string]any{
		"reconcile": map[string]any{
			"resources": placeholders.StringSliceDefault(crdKinds),
		},
	}
	spec := buildSchemaSpec(gs, fmt.Sprintf("ack-%s-controller", gs.Service), values)

	return Schema{
		APIVersion: "v1alpha1",
		Kind:       serviceUpper + "crdgraph",
		Spec:       spec,
	}
}
