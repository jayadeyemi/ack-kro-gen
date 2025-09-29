package placeholders

import (
	"strings"
	"testing"
)

const sample = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: __KRO_SA_NAME__
  namespace: __KRO_NS__
  annotations:
    eks.amazonaws.com/role-arn: "__KRO_IRSA_ARN__"
`

func TestReplaceYAMLScalars(t *testing.T) {
	out, err := ReplaceYAMLScalars(sample)
	if err != nil { t.Fatal(err) }
	checks := []string{
		"${schema.spec.values.serviceAccount.name}",
		"${schema.spec.namespace}",
		"${ackIamRole.status.ackResourceMetadata.arn}",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in output: %s", want, out)
		}
	}
}