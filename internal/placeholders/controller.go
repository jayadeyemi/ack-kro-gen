package placeholders

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jayadeyemi/ack-kro-gen/internal/config"
)

// ControllerValues builds the values block for the controller graph schema using
// chart defaults and GraphSpec overrides.
func ControllerValues(gs config.GraphSpec, overrides map[string]any) map[string]any {
	// Seed with full set of controller defaults derived from SchemaDefaults.
	// Chart defaults are not provided here, so pass nil.
	values, _ := controllerDefaults(gs, nil)

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

	tagFallback := DefaultTag(gs)
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

// schemaDefaultValue returns the raw default string for a given schema path
// like "aws.accountID" by looking up SchemaDefaults.
func schemaDefaultValue(path string) string {
	key := "${schema.spec." + strings.TrimSpace(path) + "}"
	if v, ok := SchemaDefaults[key]; ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// schemaDefaultBool returns the boolean value of a schema default.
func schemaDefaultBool(path string) bool {
	v := strings.ToLower(strings.TrimSpace(schemaDefaultValue(path)))
	return v == "true"
}

// DefaultRepo returns the default ACK controller image repository for a service name.
func DefaultRepo(serviceName string) string {
	svc := strings.TrimSpace(serviceName)
	if svc == "" {
		return ""
	}
	return fmt.Sprintf("public.ecr.aws/aws-controllers-k8s/%s-controller", strings.ToLower(svc))
}

// DefaultTag returns the default controller image tag, falling back to the graph version.
func DefaultTag(gs config.GraphSpec) string {
	if v := strings.TrimSpace(gs.Image.Tag); v != "" {
		return v
	}
	return strings.TrimSpace(gs.Version)
}

func controllerDefaults(gs config.GraphSpec, chartDefaults map[string]any) (map[string]any, map[string]string) {
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

	rawDefaults := resolveControllerDefaults(gs, chartDefaults)

	values := map[string]any{}
	for key, raw := range rawDefaults {
		if _, skip := skipPaths[key]; skip {
			continue
		}
		segments := strings.Split(key, ".")
		if len(segments) == 0 {
			continue
		}
		if _, ok := allowedRoots[segments[0]]; !ok {
			continue
		}
		setNestedValue(values, segments, formatSchemaDefault(segments, raw))
	}

	return values, rawDefaults
}

func resolveControllerDefaults(gs config.GraphSpec, chartDefaults map[string]any) map[string]string {
	resolved := map[string]string{}
	flattened := map[string]string{}
	if chartDefaults != nil {
		flattenChartDefaults(nil, chartDefaults, flattened)
	}

	for schemaRef, fallback := range SchemaDefaults {
		segments := schemaPathSegments(schemaRef)
		if len(segments) == 0 {
			continue
		}
		key := strings.Join(segments, ".")
		if val, ok := flattened[key]; ok {
			resolved[key] = strings.TrimSpace(val)
			continue
		}
		resolved[key] = strings.TrimSpace(resolveTokens(fallback, gs))
	}

	return resolved
}

func flattenChartDefaults(prefix []string, value any, out map[string]string) {
	// Recurse only for nested maps. Treat everything else as a scalar.
	if m, ok := value.(map[string]any); ok {
		for k, child := range m {
			flattenChartDefaults(append(prefix, k), child, out)
		}
		return
	}

	key := strings.Join(prefix, ".")
	if key == "" {
		return
	}
	out[key] = marshalScalar(value)
}

func marshalScalar(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", t)
	case float32, float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", t), "0"), ".")
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(b)
	}
}

func resolveTokens(in string, gs config.GraphSpec) string {
	out := in
	replacements := map[string]string{
		"_CONTROLLER_NAME_":    defaultControllerName(gs),
		"_CONTROLLER_SERVICE_": defaultControllerName(gs),
		"_IMAGE_REPOSITORY_":   defaultImageRepository(gs),
		"_IMAGE_TAG_":          defaultImageTag(gs),
		"_CONTROLLER_VERSION_": defaultImageTag(gs),
		"_NAMESPACE_":          defaultNamespace(gs),
		"_SERVICE_LOWER_":      strings.ToLower(strings.TrimSpace(gs.Service)),
	}
	for token, val := range replacements {
		if val == "" {
			val = ""
		}
		out = strings.ReplaceAll(out, token, val)
	}
	return out
}

func defaultControllerName(gs config.GraphSpec) string {
	svc := strings.TrimSpace(gs.Service)
	if svc == "" {
		return "ack-controller"
	}
	return fmt.Sprintf("ack-%s-controller", strings.ToLower(svc))
}

func defaultNamespace(gs config.GraphSpec) string {
	if ns := strings.TrimSpace(gs.Namespace); ns != "" {
		return ns
	}
	return "ack-system"
}

func defaultImageRepository(gs config.GraphSpec) string {
	svc := strings.TrimSpace(gs.Service)
	if svc == "" {
		return ""
	}
	return fmt.Sprintf("public.ecr.aws/aws-controllers-k8s/%s-controller", strings.ToLower(svc))
}

func defaultImageTag(gs config.GraphSpec) string {
	return strings.TrimSpace(gs.Version)
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
		if !strings.HasPrefix(val, "[") {
			val = "[]"
		}
		return "string[] | default=" + val
	case "object":
		if val == "" {
			val = "{}"
		}
		if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
			return "object | default={}" // arrays are coerced to empty object for spec defaults
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
