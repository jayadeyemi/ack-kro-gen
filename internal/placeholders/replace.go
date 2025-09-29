package placeholders

import (
	"bytes"
	"strings"

	"gopkg.in/yaml.v3"
)

var sentinelToSchema = map[string]string{
	"__KRO_NAME__":       "${schema.spec.name}",
	"__KRO_NS__":         "${schema.spec.namespace}",
	"__KRO_IMAGE_REPO__": "${schema.spec.values.image.repository}",
	"__KRO_IMAGE_TAG__":  "${schema.spec.values.image.tag}",
	"__KRO_SA_NAME__":    "${schema.spec.values.serviceAccount.name}",
	"__KRO_IRSA_ARN__":   "${ackIamRole.status.ackResourceMetadata.arn}",
	"__KRO_AWS_REGION__": "${schema.spec.values.aws.region}",
	"__KRO_LOG_LEVEL__":  "${schema.spec.values.log.level}",
	"__KRO_LOG_DEV__":    "${schema.spec.values.log.enabled}",
}

// ReplaceYAMLScalars walks the YAML AST and replaces sentinel strings inside scalar nodes only.
func ReplaceYAMLScalars(in string) (string, error) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(in), &root); err != nil { return "", err }
	walk(&root)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil { return "", err }
	_ = enc.Close()
	return buf.String(), nil
}

func walk(n *yaml.Node) {
	if n == nil { return }
	if n.Kind == yaml.ScalarNode && n.Tag == "!!str" {
		for k, v := range sentinelToSchema {
			if strings.Contains(n.Value, k) {
				n.Value = strings.ReplaceAll(n.Value, k, v)
			}
		}
	}
	for _, c := range n.Content { walk(c) }
}