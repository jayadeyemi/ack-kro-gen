package placeholders

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jayadeyemi/ack-kro-gen/internal/config"
)

// ControllerValues builds the values block for the controller graph schema using
// chart defaults, ValuesSpec overrides, and observed CRD kinds.
func ControllerValues(gs config.ValuesSpec, crdKinds []string) map[string]any {
	// Seed with full set of controller defaults derived from SchemaDefaults.
	// Chart defaults are not provided here, so pass nil.
	values, _ := controllerDefaults(gs, nil)

	setString := func(path []string, value, schemaPath string) {
		v := strings.TrimSpace(value)
		if v == "" {
			return
		}
		setNestedValue(values, path, StringDefault(v, schemaDefaultValue(schemaPath)))
	}

	setBool := func(path []string, value bool, schemaPath string) {
		fallback := schemaDefaultBool(schemaPath)
		if value == fallback {
			return
		}
		setNestedValue(values, path, BoolDefault(value, fallback))
	}

	setInt := func(path []string, value int, schemaPath string) {
		if value == 0 {
			return
		}
		fallback := schemaDefaultInt(schemaPath)
		if value == fallback {
			return
		}
		setNestedValue(values, path, IntDefault(value, fallback))
	}

	setStringSlice := func(path []string, items []string) {
		if len(items) == 0 {
			return
		}
		setNestedValue(values, path, StringSliceDefault(items))
	}

	setIntMap := func(path []string, m map[string]int) {
		if len(m) == 0 {
			return
		}
		out := make(map[string]any, len(m))
		for k, v := range m {
			out[k] = v
		}
		setNestedValue(values, path, out)
	}

	setAnyMap := func(path []string, m map[string]any) {
		if len(m) == 0 {
			return
		}
		clone := make(map[string]any, len(m))
		for k, v := range m {
			clone[k] = v
		}
		setNestedValue(values, path, clone)
	}

	setSliceOfMaps := func(path []string, list []map[string]any) {
		if len(list) == 0 {
			return
		}
		clone := make([]map[string]any, len(list))
		for i, item := range list {
			elem := make(map[string]any, len(item))
			for k, v := range item {
				elem[k] = v
			}
			clone[i] = elem
		}
		setNestedValue(values, path, clone)
	}

	// AWS configuration.
	setString([]string{"aws", "region"}, gs.AWS.Region, "aws.region")
	setString([]string{"aws", "endpoint_url"}, gs.AWS.EndpointURL, "aws.endpoint_url")
	setString([]string{"aws", "credentials", "secretName"}, gs.AWS.Credentials.SecretName, "aws.credentials.secretName")
	setString([]string{"aws", "credentials", "secretKey"}, gs.AWS.Credentials.SecretKey, "aws.credentials.secretKey")
	setString([]string{"aws", "credentials", "profile"}, gs.AWS.Credentials.Profile, "aws.credentials.profile")

	// Image configuration.
	setString([]string{"image", "repository"}, gs.Image.Repository, "image.repository")
	setString([]string{"image", "tag"}, gs.Image.Tag, "image.tag")
	setString([]string{"image", "pullPolicy"}, gs.Image.PullPolicy, "image.pullPolicy")
	setStringSlice([]string{"image", "pullSecrets"}, gs.Image.PullSecrets)

	// Logging and observability.
	setBool([]string{"log", "enable_development_logging"}, gs.Log.EnableDevelopmentLogging, "log.enable_development_logging")
	setString([]string{"log", "level"}, gs.Log.Level, "log.level")

	// Namespace scoping and install scope.
	setString([]string{"watchNamespace"}, gs.WatchNamespace, "watchNamespace")
	setString([]string{"watchSelectors"}, gs.WatchSelectors, "watchSelectors")
	setString([]string{"installScope"}, gs.InstallScope, "installScope")

	// Global policy overrides.
	setStringSlice([]string{"resourceTags"}, gs.ResourceTags)
	setString([]string{"deletionPolicy"}, gs.DeletionPolicy, "deletionPolicy")

	// ServiceAccount configuration.
	setBool([]string{"serviceAccount", "create"}, gs.ServiceAccount.Create, "serviceAccount.create")
	setString([]string{"serviceAccount", "name"}, gs.ServiceAccount.Name, "serviceAccount.name")
	if len(gs.ServiceAccount.Annotations) > 0 {
		setNestedValue(values, []string{"serviceAccount", "annotations"}, MapOrDefault(gs.ServiceAccount.Annotations))
	}

	// Leader election.
	setBool([]string{"leaderElection", "enabled"}, gs.LeaderElection.Enabled, "leaderElection.enabled")
	setString([]string{"leaderElection", "namespace"}, gs.LeaderElection.Namespace, "leaderElection.namespace")

	// Metrics service.
	setBool([]string{"metrics", "service", "create"}, gs.Metrics.Service.Create, "metrics.service.create")
	setString([]string{"metrics", "service", "type"}, gs.Metrics.Service.Type, "metrics.service.type")

	// Resource requests and limits.
	if len(gs.Resources.Requests) > 0 {
		setNestedValue(values, []string{"resources", "requests"}, MapOrDefault(gs.Resources.Requests))
	}
	if len(gs.Resources.Limits) > 0 {
		setNestedValue(values, []string{"resources", "limits"}, MapOrDefault(gs.Resources.Limits))
	}

	// Role metadata.
	if len(gs.Role.Labels) > 0 {
		setNestedValue(values, []string{"role", "labels"}, MapOrDefault(gs.Role.Labels))
	}

	// Deployment tuning.
	if len(gs.Deployment.Annotations) > 0 {
		setNestedValue(values, []string{"deployment", "annotations"}, MapOrDefault(gs.Deployment.Annotations))
	}
	if len(gs.Deployment.Labels) > 0 {
		setNestedValue(values, []string{"deployment", "labels"}, MapOrDefault(gs.Deployment.Labels))
	}
	setInt([]string{"deployment", "containerPort"}, gs.Deployment.ContainerPort, "deployment.containerPort")
	setInt([]string{"deployment", "replicas"}, gs.Deployment.Replicas, "deployment.replicas")
	if len(gs.Deployment.NodeSelector) > 0 {
		setNestedValue(values, []string{"deployment", "nodeSelector"}, MapOrDefault(gs.Deployment.NodeSelector))
	}
	setSliceOfMaps([]string{"deployment", "tolerations"}, gs.Deployment.Tolerations)
	setAnyMap([]string{"deployment", "affinity"}, gs.Deployment.Affinity)
	setString([]string{"deployment", "priorityClassName"}, gs.Deployment.PriorityClassName, "deployment.priorityClassName")
	setBool([]string{"deployment", "hostNetwork"}, gs.Deployment.HostNetwork, "deployment.hostNetwork")
	setString([]string{"deployment", "dnsPolicy"}, gs.Deployment.DNSPolicy, "deployment.dnsPolicy")
	setAnyMap([]string{"deployment", "strategy"}, gs.Deployment.Strategy)
	setSliceOfMaps([]string{"deployment", "extraVolumes"}, gs.Deployment.ExtraVolumes)
	setSliceOfMaps([]string{"deployment", "extraVolumeMounts"}, gs.Deployment.ExtraVolumeMounts)
	setSliceOfMaps([]string{"deployment", "extraEnvVars"}, gs.Deployment.ExtraEnvVars)

	// Reconcile behaviour.
	setInt([]string{"reconcile", "defaultResyncPeriod"}, gs.Reconcile.DefaultResyncPeriod, "reconcile.defaultResyncPeriod")
	setInt([]string{"reconcile", "defaultMaxConcurrentSyncs"}, gs.Reconcile.DefaultMaxConcurrentSyncs, "reconcile.defaultMaxConcurrentSyncs")
	setIntMap([]string{"reconcile", "resourceResyncPeriods"}, gs.Reconcile.ResourceResyncPeriods)
	setIntMap([]string{"reconcile", "resourceMaxConcurrentSyncs"}, gs.Reconcile.ResourceMaxConcurrentSyncs)
	if len(gs.Reconcile.Resources) > 0 {
		setStringSlice([]string{"reconcile", "resources"}, gs.Reconcile.Resources)
	} else if len(crdKinds) > 0 {
		setStringSlice([]string{"reconcile", "resources"}, crdKinds)
	}

	// Feature toggles.
	setBool([]string{"enableCARM"}, gs.EnableCARM, "enableCARM")
	fgDefaults := schemaFeatureGateDefaults()
	featureGateOverrides := map[string]any{}
	if def, ok := fgDefaults["ServiceLevelCARM"]; !ok || gs.FeatureGates.ServiceLevelCARM != def {
		featureGateOverrides["ServiceLevelCARM"] = BoolDefault(gs.FeatureGates.ServiceLevelCARM, def)
	}
	if def, ok := fgDefaults["TeamLevelCARM"]; !ok || gs.FeatureGates.TeamLevelCARM != def {
		featureGateOverrides["TeamLevelCARM"] = BoolDefault(gs.FeatureGates.TeamLevelCARM, def)
	}
	if def, ok := fgDefaults["ReadOnlyResources"]; !ok || gs.FeatureGates.ReadOnlyResources != def {
		featureGateOverrides["ReadOnlyResources"] = BoolDefault(gs.FeatureGates.ReadOnlyResources, def)
	}
	if def, ok := fgDefaults["ResourceAdoption"]; !ok || gs.FeatureGates.ResourceAdoption != def {
		featureGateOverrides["ResourceAdoption"] = BoolDefault(gs.FeatureGates.ResourceAdoption, def)
	}
	if len(featureGateOverrides) > 0 {
		setNestedValue(values, []string{"featureGates"}, featureGateOverrides)
	}

	return values
}

// schemaDefaultValue returns the raw default string for a given schema path
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

func schemaDefaultInt(path string) int {
	v := strings.TrimSpace(schemaDefaultValue(path))
	if v == "" {
		return 0
	}
	if i, err := strconv.Atoi(v); err == nil {
		return i
	}
	return 0
}

func schemaFeatureGateDefaults() map[string]bool {
	raw := strings.TrimSpace(schemaDefaultValue("featureGates"))
	if raw == "" {
		return map[string]bool{}
	}
	out := map[string]bool{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return map[string]bool{}
	}
	return out
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
func DefaultTag(gs config.ValuesSpec) string {
	if v := strings.TrimSpace(gs.Image.Tag); v != "" {
		return v
	}
	return strings.TrimSpace(gs.Version)
}

func controllerDefaults(gs config.ValuesSpec, chartDefaults map[string]any) (map[string]any, map[string]string) {
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

func resolveControllerDefaults(gs config.ValuesSpec, chartDefaults map[string]any) map[string]string {
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

func resolveTokens(in string, gs config.ValuesSpec) string {
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

func defaultControllerName(gs config.ValuesSpec) string {
	svc := strings.TrimSpace(gs.Service)
	if svc == "" {
		return "ack-controller"
	}
	return fmt.Sprintf("ack-%s-controller", strings.ToLower(svc))
}

func defaultNamespace(gs config.ValuesSpec) string {
	if ns := strings.TrimSpace(gs.Namespace); ns != "" {
		return ns
	}
	return "ack-system"
}

func defaultImageRepository(gs config.ValuesSpec) string {
	svc := strings.TrimSpace(gs.Service)
	if svc == "" {
		return ""
	}
	return fmt.Sprintf("public.ecr.aws/aws-controllers-k8s/%s-controller", strings.ToLower(svc))
}

func defaultImageTag(gs config.ValuesSpec) string {
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
	case strings.HasSuffix(lower, "replicas"), strings.HasSuffix(lower, "containerport"), strings.HasSuffix(lower, "defaultmaxconcurrentsyncs"), strings.HasSuffix(lower, "maxsessionduration"), strings.HasSuffix(lower, "defaultresyncperiod"):
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

func BoolDefault(v bool, fallback bool) string {
	val := v
	if v == fallback {
		val = fallback
	}
	if val {
		return "boolean | default=true"
	}
	return "boolean | default=false"
}

func IntDefault(v, _ int) string {
	return fmt.Sprintf("integer | default=%d", v)
}

// StringSliceDefault formats a string slice default while de-duplicating entries.
func StringSliceDefault(items []string) string {
	if len(items) == 0 {
		return `string[] | default=[]`
	}
	vals := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		s := strings.TrimSpace(item)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		vals = append(vals, fmt.Sprintf("\"%s\"", s))
	}
	if len(vals) == 0 {
		return `string[] | default=[]`
	}
	return "string[] | default=[" + strings.Join(vals, ",") + "]"
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
