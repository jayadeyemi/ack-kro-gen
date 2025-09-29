package classify

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Obj struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string
	RawYAML    string
}

type Groups struct {
	CRDs        []Obj
	Core        []Obj // ServiceAccount, Service, ConfigMap, Namespace
	RBAC        []Obj // ClusterRole, Role, ClusterRoleBinding, RoleBinding
	Deployments []Obj // Deployments
	Others      []Obj // any leftover kinds
}

func Parse(doc string) (Obj, error) {
	var m map[string]any
	if err := yaml.Unmarshal([]byte(doc), &m); err != nil {
		return Obj{}, err
	}
	apiv, _ := m["apiVersion"].(string)
	kind, _ := m["kind"].(string)
	md, _ := m["metadata"].(map[string]any)
	name, _ := md["name"].(string)
	ns, _ := md["namespace"].(string)
	return Obj{APIVersion: apiv, Kind: kind, Name: name, Namespace: ns, RawYAML: strings.TrimSpace(doc) + "\n"}, nil
}

func Classify(objs []Obj) Groups {
	g := Groups{}
	for _, o := range objs {
		if strings.HasPrefix(o.APIVersion, "apiextensions.k8s.io/") && o.Kind == "CustomResourceDefinition" {
			g.CRDs = append(g.CRDs, o); continue
		}
		switch o.Kind {
		case "ServiceAccount", "Service", "ConfigMap", "Namespace":
			g.Core = append(g.Core, o)
		case "ClusterRole", "Role", "ClusterRoleBinding", "RoleBinding":
			g.RBAC = append(g.RBAC, o)
		case "Deployment":
			g.Deployments = append(g.Deployments, o)
		default:
			g.Others = append(g.Others, o)
		}
	}
	// Deterministic order within groups
	less := func(a, b Obj) bool {
		ka := fmt.Sprintf("%s/%s/%s", a.Kind, a.Namespace, a.Name)
		kb := fmt.Sprintf("%s/%s/%s", b.Kind, b.Namespace, b.Name)
		return ka < kb
	}
	sort.Slice(g.CRDs, func(i, j int) bool { return less(g.CRDs[i], g.CRDs[j]) })
	sort.Slice(g.Core, func(i, j int) bool { return less(g.Core[i], g.Core[j]) })
	sort.Slice(g.RBAC, func(i, j int) bool { return less(g.RBAC[i], g.RBAC[j]) })
	sort.Slice(g.Deployments, func(i, j int) bool { return less(g.Deployments[i], g.Deployments[j]) })
	sort.Slice(g.Others, func(i, j int) bool { return less(g.Others[i], g.Others[j]) })
	return g
}