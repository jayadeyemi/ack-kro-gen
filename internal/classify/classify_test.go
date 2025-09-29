package classify

import "testing"

func TestClassifyOrder(t *testing.T) {
	crd := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: a.example.com
`
	sa := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: s
`
	rb := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: r
`
	dep := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: d
`

	objs := []Obj{}
	for _, d := range []string{crd, sa, rb, dep} {
		o, err := Parse(d)
		if err != nil { t.Fatal(err) }
		objs = append(objs, o)
	}
	g := Classify(objs)
	if len(g.CRDs) != 1 || len(g.Core) != 1 || len(g.RBAC) != 1 || len(g.Deployments) != 1 {
		t.Fatalf("unexpected grouping: %+v", g)
	}
}