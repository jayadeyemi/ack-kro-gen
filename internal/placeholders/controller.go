package placeholders

import (
	"fmt"
	"strings"

	"github.com/jayadeyemi/ack-kro-gen/internal/config"
)

// ControllerValues builds the values block for the controller graph schema.
func ControllerValues(gs config.GraphSpec, overrides map[string]any) map[string]any {
	values := defaultControllerValues()

	serviceName := strings.TrimSpace(gs.Service)

	setNestedValue(values, []string{"aws", "accountID"}, StringDefault(gs.AWS.AccountID, schemaDefaultValue("aws.accountID")))
	setNestedValue(values, []string{"aws", "region"}, StringDefault(gs.AWS.Region, schemaDefaultValue("aws.region")))
	setNestedValue(values, []string{"aws", "credentials", "secretName"}, StringDefault(gs.AWS.SecretName, schemaDefaultValue("aws.credentials.secretName")))
	setNestedValue(values, []string{"aws", "credentials", "secretKey"}, StringDefault(gs.AWS.Credentials, schemaDefaultValue("aws.credentials.secretKey")))
	setNestedValue(values, []string{"aws", "credentials", "profile"}, StringDefault(gs.AWS.Profile, schemaDefaultValue("aws.credentials.profile")))

	setNestedValue(values, []string{"log", "enable_development_logging"}, BoolDefault(gs.Controller.LogDev, schemaDefaultBool("log.enable_development_logging")))
	setNestedValue(values, []string{"log", "level"}, StringDefault(gs.Controller.LogLevel, schemaDefaultValue("log.level")))

	setNestedValue(values, []string{"watchNamespace"}, StringDefault(gs.Controller.WatchNamespace, schemaDefaultValue("watchNamespace")))

	repoFallback := DefaultRepo(serviceName)
	if repoFallback == "" {
		repoFallback = schemaDefaultValue("image.repository")
	}
	setNestedValue(values, []string{"image", "repository"}, StringDefault(gs.Image.Repository, repoFallback))

	tagFallback := DefaultTag()
	if tagFallback == "" {
		tagFallback = schemaDefaultValue("image.tag")
	}
	setNestedValue(values, []string{"image", "tag"}, StringDefault(gs.Image.Tag, tagFallback))

	saFallback := schemaDefaultValue("serviceAccount.name")
	if serviceName != "" {
		saFallback = fmt.Sprintf("ack-%s-controller", serviceName)
	}
	setNestedValue(values, []string{"serviceAccount", "name"}, StringDefault(gs.ServiceAccount.Name, saFallback))
	setNestedValue(values, []string{"serviceAccount", "annotations"}, MapOrDefault(gs.ServiceAccount.Annotations))

	setNestedValue(values, []string{"leaderElection", "namespace"}, StringDefault(gs.Namespace, schemaDefaultValue("leaderElection.namespace")))

	roleFallback := schemaDefaultValue("iamRole.roleDescription")
	if serviceName != "" {
		roleFallback = fmt.Sprintf("IRSA role for ACK %s controller deployment on EKS cluster using KRO Resource Graph", strings.ToLower(serviceName))
	}
	setNestedValue(values, []string{"iamRole", "roleDescription"}, StringDefault("", roleFallback))

	if len(overrides) > 0 {
		values["overrides"] = overrides
	}

	return values
}

func defaultControllerValues() map[string]any {
	allowedRoots := map[string]struct{}{
		"aws":            {},
		"deletionPolicy": {},
		"deployment":     {},
		"resources":      {},
		"role":           {},
		"metrics":        {},
		"log":            {},
		"installScope":   {},
		"watchNamespace": {},
		"watchSelectors": {},
		"resourceTags":   {},
		"reconcile":      {},
		"enableCARM":     {},
		"featureGates":   {},
		"serviceAccount": {},
		"leaderElection": {},
		"iamRole":        {},
		"image":          {},
	}

	skipPaths := map[string]struct{}{
		"aws.endpoint_url": {},
	}

	values := map[string]any{}

	for schemaRef, def := range SchemaDefaults {
		path := schemaPathSegments(schemaRef)
		if len(path) == 0 {
			continue
		}
		if _, ok := allowedRoots[path[0]]; !ok {
			continue
		}
		joined := strings.Join(path, ".")
		if _, skip := skipPaths[joined]; skip {
			continue
		}

		formatted := formatSchemaDefault(path, ApplySentinelToSchema(def))
		setNestedValue(values, path, formatted)
	}

	return values
}

func schemaPathSegments(schemaRef string) []string {
	ref := strings.TrimSpace(schemaRef)
	if !strings.HasPrefix(ref, "${") || !strings.HasSuffix(ref, "}") {
		return nil
	}
	ref = strings.TrimPrefix(ref, "${")
	ref = strings.TrimSuffix(ref, "}")
	if !strings.HasPrefix(ref, "schema.") {
		return nil
	}
	ref = strings.TrimPrefix(ref, "schema.")
	parts := strings.Split(ref, ".")
	for i, part := range parts {
		if part == "spec" {
			segs := make([]string, len(parts[i+1:]))
			copy(segs, parts[i+1:])
			return segs
		}
	}
	return parts
}

func setNestedValue(root map[string]any, path []string, val any) {
	cur := root
	for i, part := range path {
		if i == len(path)-1 {
			cur[part] = val
			return
		}
		next, ok := cur[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[part] = next
		}
		cur = next
	}
}

func formatSchemaDefault(path []string, raw string) string {
	key := strings.Join(path, ".")
	val := strings.TrimSpace(raw)
	switch typeForPath(key) {
	case "boolean":
		if val == "" {
			val = "false"
		}
		lower := strings.ToLower(val)
		if lower != "true" && lower != "false" {
			lower = "false"
		}
		return "boolean | default=" + lower
	case "integer":
		if val == "" {
			val = "0"
		}
		return "integer | default=" + val
	case "string[]":
		if val == "" {
			val = "[]"
		}
		return "string[] | default=" + val
	case "object":
		if val == "" || val == "[]" {
			val = "{}"
		}
		return "object | default=" + val
	default:
		if val == "" {
			val = "\"\""
		}
		return "string | default=" + val
	}
}

func typeForPath(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, "create"), strings.HasSuffix(lower, "enabled"), strings.Contains(lower, "enable_"), strings.HasSuffix(lower, "hostnetwork"), strings.HasSuffix(lower, "enablecarm"):
		return "boolean"
	case strings.HasSuffix(lower, "replicas"), strings.HasSuffix(lower, "containerport"), strings.HasSuffix(lower, "defaultmaxconcurrentsyncs"), strings.HasSuffix(lower, "maxsessionduration"):
		return "integer"
	case strings.HasSuffix(lower, "pullsecrets"), strings.HasSuffix(lower, "resourcetags"), strings.HasSuffix(lower, ".resources"):
		return "string[]"
	case strings.HasSuffix(lower, "labels"), strings.HasSuffix(lower, "annotations"), strings.HasSuffix(lower, "nodeselector"), strings.HasSuffix(lower, "tolerations"), strings.HasSuffix(lower, "affinity"),
		strings.HasSuffix(lower, "strategy"), strings.Contains(lower, "extravolume"), strings.Contains(lower, "extraenv"), strings.Contains(lower, "resourceresyncperiods"),
		strings.Contains(lower, "resourcemaxconcurrentsyncs"), strings.HasSuffix(lower, "featuregates"):
		return "object"
	default:
		return "string"
	}
}

func schemaDefaultValue(path string) string {
	key := "${schema.spec." + path + "}"
	def, ok := SchemaDefaults[key]
	if !ok {
		return ""
	}
	return strings.TrimSpace(ApplySentinelToSchema(def))
}

func schemaDefaultBool(path string) bool {
	key := "${schema.spec." + path + "}"
	def := strings.TrimSpace(SchemaDefaults[key])
	return strings.EqualFold(def, "true")
}

// StringDefault returns `string | default=<value>` with empty defaults handled.
func StringDefault(v, fallback string) string {
	s := strings.TrimSpace(v)
	if s == "" {
		s = strings.TrimSpace(fallback)
	}
	if s == "" {
		return `string | default=""`
	}
	return "string | default=" + s
}

// BoolDefault returns `boolean | default=<value>` using v when valid or fallback otherwise.
func BoolDefault(v string, fallback bool) string {
	s := strings.TrimSpace(strings.ToLower(v))
	if s == "true" || s == "false" {
		return "boolean | default=" + s
	}
	if fallback {
		return "boolean | default=true"
	}
	return "boolean | default=false"
}

// MapOrDefault converts a map[string]string to map[string]any for YAML emission.
func MapOrDefault(in map[string]string) any {
	if len(in) == 0 {
		return "object | default={}"
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// DefaultRepo returns the fallback controller image repository for a service.
func DefaultRepo(service string) string {
	service = strings.TrimSpace(service)
	if service == "" {
		return ""
	}
	return "public.ecr.aws/aws-controllers-k8s/" + strings.ToLower(service) + "-controller"
}

// DefaultTag returns the fallback controller image tag.
func DefaultTag() string {
	return "latest"
}
