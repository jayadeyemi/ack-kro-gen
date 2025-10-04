package kro

import (
	"github.com/jayadeyemi/ack-kro-gen/internal/config"
	"github.com/jayadeyemi/ack-kro-gen/internal/placeholders"
)

// buildSchemaSpec standardizes schema spec construction for controller and CRD graphs,
// ensuring name and namespace use placeholder-aware defaults.
func buildSchemaSpec(gs config.ValuesSpec, fallbackName string, values map[string]any) SchemaSpec {
	spec := SchemaSpec{
		Name:      placeholders.StringDefault(gs.ReleaseName, fallbackName),
		Namespace: placeholders.StringDefault(gs.Namespace, "ack-system"),
	}
	if len(values) > 0 {
		spec.Values = values
	}
	return spec
}
