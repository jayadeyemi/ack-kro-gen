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
			Schema:    CtrlSchema(gs, serviceUpper),
			Resources: ctrlResources,
		},
	}
}

// TODO: update this block and the placeholders package. all fields must be referenced from there.
// define the controller schema
func CtrlSchema(gs config.GraphSpec, serviceUpper string) Schema {
	values := map[string]any{
		"aws": map[string]any{
			"accountID": defStr(gs.AWS.AccountID, ""),
			"region":    defStr(gs.AWS.Region, ""),
			"credentials": map[string]any{
				"secretName": defStr(gs.AWS.SecretName, ""),
				"secretKey":  defStr(gs.AWS.Credentials, "credentials"),
				"profile":    defStr(gs.AWS.Profile, "default"),
			},
		},
		"deletionPolicy": defStr("", "delete"),
		"deployment": map[string]any{
			"replicas":          "integer | default=1",
			"containerPort":     "integer | default=8080",
			"labels":            "object | default={}",
			"annotations":       "object | default={}",
			"nodeSelector":      "object | default={}",
			"tolerations":       "object | default={}",
			"affinity":          "object | default={}",
			"priorityClassName": defStr("", ""),
			"hostNetwork":       boolDefault("", false),
			"dnsPolicy":         defStr("", "ClusterFirst"),
			"strategy":          "object | default={}",
			"extraVolumes":      "object | default={}",
			"extraVolumeMounts": "object | default={}",
			"extraEnvVars":      "object | default={}",
		},
		"resources": map[string]any{
			"requests": map[string]any{
				"memory": defStr("", "64Mi"),
				"cpu":    defStr("", "50m"),
			},
			"limits": map[string]any{
				"memory": defStr("", "128Mi"),
				"cpu":    defStr("", "100m"),
			},
		},
		"role": map[string]any{
			"labels": "object | default={}",
		},
		"metrics": map[string]any{
			"service": map[string]any{
				"create": boolDefault("", true),
				"type":   defStr("", "ClusterIP"),
			},
		},
		"log": map[string]any{
			"enable_development_logging": boolDefault(gs.Controller.LogDev, false),
			"level":                      defStr(gs.Controller.LogLevel, "info"),
		},
		"installScope":   defStr("", "cluster"),
		"watchNamespace": defStr(gs.Controller.WatchNamespace, ""),
		"watchSelectors": defStr("", ""),
		"resourceTags":   "string[] | default=[]",
		"reconcile": map[string]any{
			"defaultResyncPeriod":        defStr("", "10h"),
			"defaultMaxConcurrentSyncs":  "integer | default=5",
			"resourceResyncPeriods":      "object | default={}",
			"resourceMaxConcurrentSyncs": "object | default={}",
			"resources":                  "string[] | default=[]",
		},
		"enableCARM":   boolDefault("", true),
		"featureGates": "object | default={}",
		"serviceAccount": map[string]any{
			"create":      boolDefault("", true),
			"name":        defStr(gs.ServiceAccount.Name, fmt.Sprintf("ack-%s-controller", gs.Service)),
			"annotations": mapOrDefault(gs.ServiceAccount.Annotations),
		},
		"leaderElection": map[string]any{
			"enabled":   boolDefault("", false),
			"namespace": defStr(gs.Namespace, "kro"),
		},
		"iamRole": map[string]any{
			"oidcProvider":       defStr("", ""),
			"maxSessionDuration": "integer | default=3600",
			"roleDescription":    defStr("", fmt.Sprintf("IRSA role for ACK %s controller deployment on EKS cluster using KRO Resource Graph", strings.ToLower(gs.Service))),
		},
		"image": map[string]any{
			"repository":  defStr(gs.Image.Repository, defaultRepo(gs.Service)),
			"tag":         defStr(gs.Image.Tag, defaultTag()),
			"pullPolicy":  defStr("", "IfNotPresent"),
			"pullSecrets": "string[] | default=[]",
		},
	}

	if len(gs.Extras.Values) > 0 {
		values["overrides"] = gs.Extras.Values
	}

	return Schema{
		APIVersion: "v1alpha1",
		Kind:       serviceUpper + "controller",
		Spec: SchemaSpec{
			Name:      defStr(gs.ReleaseName, fmt.Sprintf("ack-%s-controller", gs.Service)),
			Namespace: defStr(gs.Namespace, "ack-system"),
			Values:    values,
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

// defStr returns `string | default=<v>` with "" when empty.
// fb is a fallback used if v is empty; if both empty -> "".
func defStr(v, fb string) string {
	s := strings.TrimSpace(v)
	if s == "" {
		s = strings.TrimSpace(fb)
	}
	if s == "" {
		return `string | default=""`
	}
	return "string | default=" + s
}

// service repo/tag fallbacks
func defaultRepo(service string) string {
	if service == "" {
		return ""
	}
	return "public.ecr.aws/aws-controllers-k8s/" + strings.ToLower(service) + "-controller"
}
func defaultTag() string { return "latest" }

func boolDefault(v string, fb bool) string {
	s := strings.TrimSpace(strings.ToLower(v))
	if s == "true" || s == "false" {
		return "boolean | default=" + s
	}
	if fb {
		return "boolean | default=true"
	}
	return "boolean | default=false"
}

func mapOrDefault(in map[string]string) any {
	if len(in) == 0 {
		return "object | default={}"
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
