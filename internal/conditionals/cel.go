package conditionals

import (
	"fmt"
	"strings"
)

// CELBuilder helps construct complex CEL expressions for arrays and objects
type CELBuilder struct {
	parts []string
}

// NewCELBuilder creates a new CEL expression builder
func NewCELBuilder() *CELBuilder {
	return &CELBuilder{parts: []string{}}
}

// AddConditionalArray adds array elements conditionally
// Example: (condition ? [items] : [])
func (b *CELBuilder) AddConditionalArray(condition, arrayExpr string) {
	b.parts = append(b.parts, fmt.Sprintf("(%s ? %s : [])", condition, arrayExpr))
}

// AddStaticArray adds static array elements
func (b *CELBuilder) AddStaticArray(arrayExpr string) {
	b.parts = append(b.parts, arrayExpr)
}

// Build concatenates all parts with +
func (b *CELBuilder) Build() string {
	if len(b.parts) == 0 {
		return "[]"
	}
	return strings.Join(b.parts, " + ")
}

// BuildDeploymentArgs creates CEL for deployment args with conditionals
func BuildDeploymentArgs() string {
	b := NewCELBuilder()

	// Base args (always present)
	b.AddStaticArray(`[
    "--aws-region", schema.spec.aws.region,
    "--aws-endpoint-url", schema.spec.aws.endpoint_url,
    "--log-level", schema.spec.log.level,
    "--resource-tags", schema.spec.resourceTags.join(","),
    "--watch-namespace", schema.spec.watchNamespace,
    "--watch-selectors", schema.spec.watchSelectors,
    "--reconcile-resources", schema.spec.reconcile.resources.join(","),
    "--deletion-policy", schema.spec.deletionPolicy
  ]`)

	// Conditional: development logging
	b.AddConditionalArray(
		"schema.spec.log.enable_development_logging",
		`["--enable-development-logging"]`,
	)

	// Conditional: leader election
	b.AddConditionalArray(
		"schema.spec.leaderElection.enabled",
		`["--enable-leader-election", "--leader-election-namespace", schema.spec.leaderElection.namespace]`,
	)

	// Conditional: resync period
	b.AddConditionalArray(
		"int(schema.spec.reconcile.defaultResyncPeriod) > 0",
		`["--reconcile-default-resync-seconds", string(schema.spec.reconcile.defaultResyncPeriod)]`,
	)

	// Conditional: max concurrent syncs
	b.AddConditionalArray(
		"int(schema.spec.reconcile.defaultMaxConcurrentSyncs) > 0",
		`["--reconcile-default-max-concurrent-syncs", string(schema.spec.reconcile.defaultMaxConcurrentSyncs)]`,
	)

	// Conditional: feature gates
	b.AddConditionalArray(
		`has(schema.spec.featureGates) && schema.spec.featureGates != ""`,
		`["--feature-gates", schema.spec.featureGates]`,
	)

	// Add enable-carm flag
	b.AddStaticArray(`["--enable-carm=" + string(schema.spec.enableCARM)]`)

	return b.Build()
}

// BuildVolumes creates CEL for conditional volumes
func BuildVolumes() string {
	return strings.TrimSpace(`
(schema.spec.aws.credentials.secretName != "" ? 
  [{
    "name": schema.spec.aws.credentials.secretName,
    "secret": {"secretName": schema.spec.aws.credentials.secretName}
  }] : []
) +
(has(schema.spec.deployment.extraVolumes) ? schema.spec.deployment.extraVolumes : [])
`)
}

// BuildVolumeMounts creates CEL for conditional volume mounts
func BuildVolumeMounts() string {
	return strings.TrimSpace(`
(schema.spec.aws.credentials.secretName != "" ? 
  [{
    "name": schema.spec.aws.credentials.secretName,
    "mountPath": "/var/run/secrets/aws",
    "readOnly": true
  }] : []
) +
(has(schema.spec.deployment.extraVolumeMounts) ? schema.spec.deployment.extraVolumeMounts : [])
`)
}

// BuildEnvVars creates CEL for conditional environment variables
func BuildEnvVars() string {
	b := NewCELBuilder()

	// Base environment variables (always present)
	b.AddStaticArray(`[
    {"name": "ACK_SYSTEM_NAMESPACE", "valueFrom": {"fieldRef": {"fieldPath": "metadata.namespace"}}},
    {"name": "AWS_REGION", "value": schema.spec.aws.region},
    {"name": "AWS_ENDPOINT_URL", "value": schema.spec.aws.endpoint_url},
    {"name": "ACK_WATCH_NAMESPACE", "value": schema.spec.watchNamespace},
    {"name": "ACK_WATCH_SELECTORS", "value": schema.spec.watchSelectors},
    {"name": "RECONCILE_RESOURCES", "value": schema.spec.reconcile.resources.join(",")},
    {"name": "DELETION_POLICY", "value": schema.spec.deletionPolicy},
    {"name": "LEADER_ELECTION_NAMESPACE", "value": schema.spec.leaderElection.namespace},
    {"name": "ACK_LOG_LEVEL", "value": schema.spec.log.level},
    {"name": "ACK_RESOURCE_TAGS", "value": schema.spec.resourceTags.join(",")}
  ]`)

	// Conditional: resync period
	b.AddConditionalArray(
		"int(schema.spec.reconcile.defaultResyncPeriod) > 0",
		`[{"name": "RECONCILE_DEFAULT_RESYNC_SECONDS", "value": string(schema.spec.reconcile.defaultResyncPeriod)}]`,
	)

	// Conditional: max concurrent syncs
	b.AddConditionalArray(
		"int(schema.spec.reconcile.defaultMaxConcurrentSyncs) > 0",
		`[{"name": "RECONCILE_DEFAULT_MAX_CONCURRENT_SYNCS", "value": string(schema.spec.reconcile.defaultMaxConcurrentSyncs)}]`,
	)

	// Conditional: feature gates
	b.AddConditionalArray(
		`has(schema.spec.featureGates) && schema.spec.featureGates != ""`,
		`[{"name": "FEATURE_GATES", "value": schema.spec.featureGates}]`,
	)

	// Conditional: AWS credentials
	b.AddConditionalArray(
		`schema.spec.aws.credentials.secretName != ""`,
		`[
      {"name": "AWS_SHARED_CREDENTIALS_FILE", "value": "/var/run/secrets/aws/" + schema.spec.aws.credentials.secretKey},
      {"name": "AWS_PROFILE", "value": schema.spec.aws.credentials.profile}
    ]`,
	)

	// Conditional: extra env vars
	b.AddConditionalArray(
		`has(schema.spec.deployment.extraEnvVars) && size(schema.spec.deployment.extraEnvVars) > 0`,
		`schema.spec.deployment.extraEnvVars`,
	)

	return b.Build()
}

// BuildEnvVarsFromMap creates CEL for map-based env vars (e.g., resourceResyncPeriods)
func BuildEnvVarsFromMap(mapPath, nameTemplate, valueTemplate string) string {
	return fmt.Sprintf(`%s.map(k, v, {
  "name": %s,
  "value": %s
})`, mapPath, nameTemplate, valueTemplate)
}

// BuildConditionalField creates a CEL ternary for conditional field values
func BuildConditionalField(condition, trueValue, falseValue string) string {
	return fmt.Sprintf("%s ? %s : %s", condition, trueValue, falseValue)
}

// BuildHasField creates a CEL expression to check field existence
func BuildHasField(fieldPath string) string {
	return fmt.Sprintf("has(%s)", fieldPath)
}

// BuildNonEmptyString creates CEL to check if a string field is non-empty
func BuildNonEmptyString(fieldPath string) string {
	return fmt.Sprintf(`%s != ""`, fieldPath)
}

// BuildArrayJoin creates CEL to join array elements
func BuildArrayJoin(arrayPath, separator string) string {
	return fmt.Sprintf(`%s.join("%s")`, arrayPath, separator)
}
