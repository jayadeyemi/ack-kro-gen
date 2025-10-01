package kro

import (
	"fmt"
	"strings"
	"reflect"
	"strconv"

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

// define the controller schema 



// BuildSchemaDefaults uses reflection to flatten DefaultValues into
// a map of "${schema.spec.*}" → string defaults.
func BuildSchemaDefaults() map[string]string {
	out := make(map[string]string)
	prefix := "schema.spec"
	flattenStruct(reflect.ValueOf(DefaultValues), prefix, out)
	return out
}

// SchemaDefaults is built once at init
var SchemaDefaults = BuildSchemaDefaults()

// flattenStruct recursively walks through struct fields to build keys.
func flattenStruct(v reflect.Value, prefix string, out map[string]string) {
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		ft := t.Field(i)

		// Use field name as lowerCamel
		key := lowerFirst(ft.Name)
		path := prefix + "." + key

		switch field.Kind() {
		case reflect.Struct:
			flattenStruct(field, path, out)
		case reflect.String:
			out["${"+path+"}"] = field.String()
		case reflect.Int, reflect.Int64, reflect.Int32:
			out["${"+path+"}"] = strconv.FormatInt(field.Int(), 10)
		case reflect.Bool:
			out["${"+path+"}"] = strconv.FormatBool(field.Bool())
		case reflect.Slice:
			// naive slice → string
			if field.Len() == 0 {
				out["${"+path+"}"] = "[]"
			} else {
				var s string
				for j := 0; j < field.Len(); j++ {
					if j > 0 {
						s += ","
					}
					s += fmt.Sprintf("%v", field.Index(j).Interface())
				}
				out["${"+path+"}"] = "[" + s + "]"
			}
		case reflect.Map:
			if field.Len() == 0 {
				out["${"+path+"}"] = "{}"
			} else {
				out["${"+path+"}"] = fmt.Sprintf("%v", field.Interface())
			}
		default:
			out["${"+path+"}"] = fmt.Sprintf("%v", field.Interface())
		}
	}
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]|0x20) + s[1:]
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


