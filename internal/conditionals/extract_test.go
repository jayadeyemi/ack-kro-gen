package conditionals

import (
	"testing"
)

func TestConvertToCEL(t *testing.T) {
	tests := []struct {
		name     string
		helmExpr string
		want     string
	}{
		{
			name:     "simple boolean",
			helmExpr: ".Values.serviceAccount.create",
			want:     "schema.spec.values.serviceAccount.create",
		},
		{
			name:     "gt comparison",
			helmExpr: "gt (int .Values.replicas) 1",
			want:     "int(schema.spec.values.replicas) > 1",
		},
		{
			name:     "eq string comparison",
			helmExpr: `eq .Values.installScope "namespace"`,
			want:     `schema.spec.values.installScope == "namespace"`,
		},
		{
			name:     "or condition",
			helmExpr: "or .Values.foo .Values.bar",
			want:     "schema.spec.values.foo || schema.spec.values.bar",
		},
		{
			name:     "and condition",
			helmExpr: "and .Values.foo .Values.bar",
			want:     "schema.spec.values.foo && schema.spec.values.bar",
		},
		{
			name:     "leader election enabled",
			helmExpr: ".Values.leaderElection.enabled",
			want:     "schema.spec.values.leaderElection.enabled",
		},
		{
			name:     "metrics service create",
			helmExpr: ".Values.metrics.service.create",
			want:     "schema.spec.values.metrics.service.create",
		},
		{
			name:     "complex or with values",
			helmExpr: "or .Values.aws.credentials.secretName .Values.deployment.extraVolumes",
			want:     "schema.spec.values.aws.credentials.secretName || schema.spec.values.deployment.extraVolumes",
		},
		{
			name:     "not equals",
			helmExpr: `ne .Values.installScope "cluster"`,
			want:     `schema.spec.values.installScope != "cluster"`,
		},
		{
			name:     "less than",
			helmExpr: "lt (int .Values.replicas) 10",
			want:     "int(schema.spec.values.replicas) < 10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertToCEL(tt.helmExpr)
			if got != tt.want {
				t.Errorf("ConvertToCEL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSimplifyCondition(t *testing.T) {
	tests := []struct {
		name    string
		celExpr string
		want    string
	}{
		{
			name:    "remove == true",
			celExpr: "schema.spec.values.serviceAccount.create == true",
			want:    "schema.spec.values.serviceAccount.create",
		},
		{
			name:    "remove != false",
			celExpr: "schema.spec.values.enabled != false",
			want:    "schema.spec.values.enabled",
		},
		{
			name:    "double negation",
			celExpr: "! ! schema.spec.values.foo",
			want:    "schema.spec.values.foo",
		},
		{
			name:    "no simplification needed",
			celExpr: "schema.spec.values.replicas > 1",
			want:    "schema.spec.values.replicas > 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SimplifyCondition(tt.celExpr)
			if got != tt.want {
				t.Errorf("SimplifyCondition() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractResourceKind(t *testing.T) {
	tests := []struct {
		name  string
		block string
		want  string
	}{
		{
			name: "ServiceAccount",
			block: `apiVersion: v1
kind: ServiceAccount
metadata:
  name: test`,
			want: "ServiceAccount",
		},
		{
			name: "Role",
			block: `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: leader-election`,
			want: "Role",
		},
		{
			name: "Service",
			block: `apiVersion: v1
kind: Service
spec:
  type: ClusterIP`,
			want: "Service",
		},
		{
			name:  "no kind",
			block: `apiVersion: v1\nmetadata:\n  name: test`,
			want:  "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractResourceKind(tt.block)
			if got != tt.want {
				t.Errorf("extractResourceKind() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractConditions(t *testing.T) {
	template := `
{{- if .Values.serviceAccount.create }}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa
{{- end }}

{{ if .Values.leaderElection.enabled }}
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: leader-election
{{- end }}
`

	conditions := ExtractConditions("test-template.yaml", template)

	if len(conditions) != 2 {
		t.Errorf("Expected 2 conditions, got %d", len(conditions))
	}

	// Check first condition
	if conditions[0].ResourceID != "ServiceAccount" {
		t.Errorf("First condition ResourceID = %q, want %q", conditions[0].ResourceID, "ServiceAccount")
	}

	if conditions[0].HelmExpr != ".Values.serviceAccount.create" {
		t.Errorf("First condition HelmExpr = %q, want %q", conditions[0].HelmExpr, ".Values.serviceAccount.create")
	}

	// Check second condition
	if conditions[1].ResourceID != "Role" {
		t.Errorf("Second condition ResourceID = %q, want %q", conditions[1].ResourceID, "Role")
	}

	if conditions[1].HelmExpr != ".Values.leaderElection.enabled" {
		t.Errorf("Second condition HelmExpr = %q, want %q", conditions[1].HelmExpr, ".Values.leaderElection.enabled")
	}
}
