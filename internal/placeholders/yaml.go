// Package placeholders/yaml implements Phase 2 of the placeholder transformation:
// converting sentinel tokens to KRO schema references while preserving YAML structure.
//
// ReplaceYAMLScalars walks the YAML node tree and applies ApplySentinelToSchema to
// every scalar value, converting tokens like "_NAME_" to "${schema.spec.name}".
// This ensures only scalar values are transformed, preserving YAML structure.

package placeholders

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// ReplaceYAMLScalars walks every scalar node inside the rendered YAML and applies
// the sentinel → schema placeholder translation in a structure-preserving way.
// It returns the mutated YAML string using the same indentation settings as the
// caller used when encoding.
func ReplaceYAMLScalars(in string) (string, error) {
	if strings.TrimSpace(in) == "" {
		return in, nil
	}

	var root yaml.Node
	if err := yaml.Unmarshal([]byte(in), &root); err != nil {
		return "", err
	}

	applyScalarReplace(&root)

	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		enc.Close()
		return "", err
	}
	_ = enc.Close()
	return buf.String(), nil
}

func applyScalarReplace(n *yaml.Node) {
	if n == nil {
		return
	}
	if n.Kind == yaml.ScalarNode {
		n.Value = ReplaceAll(n.Value, false, nil)
	}
	for _, child := range n.Content {
		applyScalarReplace(child)
	}
}
