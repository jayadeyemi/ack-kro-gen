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
func ControllerValues(gs config.ValuesSpec, crdKinds []string, chartDefaults map[string]any) map[string]any {
	rawDefaults := controllerDefaults(gs, chartDefaults)
	values := buildControllerDefaultTree(rawDefaults)
	defaults := newSchemaDefaults(rawDefaults)

	setString := func(path []string, value, schemaPath string) {
		v := strings.TrimSpace(value)
		if v == "" {
			return
		}
		setNestedValue(values, path, StringDefault(v, defaults.string(schemaPath)))
	}

	setBool := func(path []string, value bool, schemaPath string) {
		fallback := defaults.bool(schemaPath)
		if value == fallback {
			return
		}
		setNestedValue(values, path, BoolDefault(value, fallback))
	}

	setInt := func(path []string, value int, schemaPath string) {
		if value == 0 {
			return
		}
		fallback := defaults.int(schemaPath)
		if value == fallback {
			return
		}
		setNestedValue(values, path, IntDefault(value, fallback))
	}

	setStringSlice := func(path []string, items []string, schemaPath string) {
		normalized := normalizeStringSlice(items)
		fallback := normalizeStringSlice(defaults.stringSlice(schemaPath))
		if len(normalized) == 0 {
			return
		}
		if slicesEqual(normalized, fallback) {
			return
		}
		setNestedValue(values, path, StringSliceDefault(normalized))
	}

	setIntMap := func(path []string, m map[string]int, schemaPath string) {
		if len(m) == 0 {
			return
		}
		fallback := defaults.intMap(schemaPath)
		if intMapsEqual(m, fallback) {
			return
		}
		out := make(map[string]any, len(m))
		for k, v := range m {
			out[k] = v
		}
		setNestedValue(values, path, out)
	}

	setAnyMap := func(path []string, m map[string]any, schemaPath string) {
		if len(m) == 0 {
			return
		}
		if mapsJSONEqual(m, defaults.anyMap(schemaPath)) {
			return
		}
		clone := make(map[string]any, len(m))
		for k, v := range m {
			clone[k] = v
		}
		setNestedValue(values, path, clone)
	}



	setStringMap := func(path []string, m map[string]string, schemaPath string) {
		if len(m) == 0 {
			return
		}
		if stringMapsEqual(m, defaults.stringMap(schemaPath)) {
			return
		}
		setNestedValue(values, path, MapOrDefault(m))
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
	setStringSlice([]string{"image", "pullSecrets"}, gs.Image.PullSecrets, "image.pullSecrets")

	// Logging and observability.
	setBool([]string{"log", "enable_development_logging"}, gs.Log.EnableDevelopmentLogging, "log.enable_development_logging")
	setString([]string{"log", "level"}, gs.Log.Level, "log.level")

	// Namespace scoping and install scope.
	setString([]string{"watchNamespace"}, gs.WatchNamespace, "watchNamespace")
	setString([]string{"watchSelectors"}, gs.WatchSelectors, "watchSelectors")
	setString([]string{"installScope"}, gs.InstallScope, "installScope")

	// Global policy overrides.
	setStringSlice([]string{"resourceTags"}, gs.ResourceTags, "resourceTags")
	setString([]string{"deletionPolicy"}, gs.DeletionPolicy, "deletionPolicy")

	// ServiceAccount configuration.
	setBool([]string{"serviceAccount", "create"}, gs.ServiceAccount.Create, "serviceAccount.create")
	setString([]string{"serviceAccount", "name"}, gs.ServiceAccount.Name, "serviceAccount.name")
	setStringMap([]string{"serviceAccount", "annotations"}, gs.ServiceAccount.Annotations, "serviceAccount.annotations")

	// Leader election.
	setBool([]string{"leaderElection", "enabled"}, gs.LeaderElection.Enabled, "leaderElection.enabled")
	setString([]string{"leaderElection", "namespace"}, gs.LeaderElection.Namespace, "leaderElection.namespace")

	// Metrics service.
	setBool([]string{"metrics", "service", "create"}, gs.Metrics.Service.Create, "metrics.service.create")
	setString([]string{"metrics", "service", "type"}, gs.Metrics.Service.Type, "metrics.service.type")

	// Resource requests and limits.
	setStringMap([]string{"resources", "requests"}, gs.Resources.Requests, "resources.requests")
	setStringMap([]string{"resources", "limits"}, gs.Resources.Limits, "resources.limits")

	// Role metadata.
	setStringMap([]string{"role", "labels"}, gs.Role.Labels, "role.labels")

	// Deployment tuning.
	setStringMap([]string{"deployment", "annotations"}, gs.Deployment.Annotations, "deployment.annotations")
	setStringMap([]string{"deployment", "labels"}, gs.Deployment.Labels, "deployment.labels")
	setInt([]string{"deployment", "containerPort"}, gs.Deployment.ContainerPort, "deployment.containerPort")
	setInt([]string{"deployment", "replicas"}, gs.Deployment.Replicas, "deployment.replicas")
	setStringMap([]string{"deployment", "nodeSelector"}, gs.Deployment.NodeSelector, "deployment.nodeSelector")
	setStringSlice([]string{"deployment", "tolerations"}, gs.Deployment.Tolerations, "deployment.tolerations")
	setAnyMap([]string{"deployment", "affinity"}, gs.Deployment.Affinity, "deployment.affinity")
	setString([]string{"deployment", "priorityClassName"}, gs.Deployment.PriorityClassName, "deployment.priorityClassName")
	setBool([]string{"deployment", "hostNetwork"}, gs.Deployment.HostNetwork, "deployment.hostNetwork")
	setString([]string{"deployment", "dnsPolicy"}, gs.Deployment.DNSPolicy, "deployment.dnsPolicy")
	setAnyMap([]string{"deployment", "strategy"}, gs.Deployment.Strategy, "deployment.strategy")
	setStringSlice([]string{"deployment", "extraVolumes"}, gs.Deployment.ExtraVolumes, "deployment.extraVolumes")
	setStringSlice([]string{"deployment", "extraVolumeMounts"}, gs.Deployment.ExtraVolumeMounts, "deployment.extraVolumeMounts")
	setStringSlice([]string{"deployment", "extraEnvVars"}, gs.Deployment.ExtraEnvVars, "deployment.extraEnvVars")

	// Reconcile behaviour.
	setInt([]string{"reconcile", "defaultResyncPeriod"}, gs.Reconcile.DefaultResyncPeriod, "reconcile.defaultResyncPeriod")
	setInt([]string{"reconcile", "defaultMaxConcurrentSyncs"}, gs.Reconcile.DefaultMaxConcurrentSyncs, "reconcile.defaultMaxConcurrentSyncs")
	setIntMap([]string{"reconcile", "resourceResyncPeriods"}, gs.Reconcile.ResourceResyncPeriods, "reconcile.resourceResyncPeriods")
	setIntMap([]string{"reconcile", "resourceMaxConcurrentSyncs"}, gs.Reconcile.ResourceMaxConcurrentSyncs, "reconcile.resourceMaxConcurrentSyncs")
	if len(gs.Reconcile.Resources) > 0 {
		setStringSlice([]string{"reconcile", "resources"}, gs.Reconcile.Resources, "reconcile.resources")
	} else {
		defaultsResources := defaults.stringSlice("reconcile.resources")
		if len(defaultsResources) > 0 {
			setStringSlice([]string{"reconcile", "resources"}, defaultsResources, "reconcile.resources")
		} else if len(crdKinds) > 0 {
			setStringSlice([]string{"reconcile", "resources"}, crdKinds, "reconcile.resources")
		}
	}

	// Feature toggles.
	setBool([]string{"enableCARM"}, gs.EnableCARM, "enableCARM")
	fgDefaults := defaults.featureGates()
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

// schemaDefaults wraps raw default strings and exposes typed helpers.
type schemaDefaults struct {
	values map[string]string
}

var controllerDefaultsAllowedRoots = map[string]struct{}{
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

var controllerDefaultsSkipPaths = map[string]struct{}{}

func newSchemaDefaults(raw map[string]string) schemaDefaults {
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		out[key] = strings.TrimSpace(v)
	}
	return schemaDefaults{values: out}
}

func (sd schemaDefaults) string(path string) string {
	if sd.values == nil {
		return ""
	}
	return strings.TrimSpace(sd.values[strings.TrimSpace(path)])
}

func (sd schemaDefaults) bool(path string) bool {
	v := strings.ToLower(sd.string(path))
	return v == "true"
}

func (sd schemaDefaults) int(path string) int {
	v := sd.string(path)
	if v == "" {
		return 0
	}
	if i, err := strconv.Atoi(v); err == nil {
		return i
	}
	return 0
}

func (sd schemaDefaults) stringSlice(path string) []string {
	raw := sd.string(path)
	if raw == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	for i, item := range out {
		out[i] = strings.TrimSpace(item)
	}
	return out
}

func (sd schemaDefaults) stringMap(path string) map[string]string {
	raw := sd.string(path)
	if raw == "" {
		return nil
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(raw), &out); err == nil {
		return out
	}
	var generic map[string]any
	if err := json.Unmarshal([]byte(raw), &generic); err != nil {
		return nil
	}
	converted := make(map[string]string, len(generic))
	for k, v := range generic {
		switch val := v.(type) {
		case string:
			converted[k] = strings.TrimSpace(val)
		case float64:
			converted[k] = strconv.FormatInt(int64(val), 10)
		case bool:
			if val {
				converted[k] = "true"
			} else {
				converted[k] = "false"
			}
		default:
			converted[k] = fmt.Sprint(val)
		}
	}
	return converted
}

func (sd schemaDefaults) intMap(path string) map[string]int {
	raw := sd.string(path)
	if raw == "" {
		return nil
	}
	var out map[string]int
	if err := json.Unmarshal([]byte(raw), &out); err == nil {
		return out
	}
	var generic map[string]any
	if err := json.Unmarshal([]byte(raw), &generic); err != nil {
		return nil
	}
	converted := make(map[string]int, len(generic))
	for k, v := range generic {
		switch val := v.(type) {
		case float64:
			converted[k] = int(val)
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
				converted[k] = parsed
			}
		}
	}
	return converted
}

func (sd schemaDefaults) anyMap(path string) map[string]any {
	raw := sd.string(path)
	if raw == "" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func (sd schemaDefaults) sliceOfMaps(path string) []map[string]any {
	raw := sd.string(path)
	if raw == "" {
		return nil
	}
	var out []map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func (sd schemaDefaults) featureGates() map[string]bool {
	raw := sd.string("featureGates")
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

func controllerDefaults(gs config.ValuesSpec, chartDefaults map[string]any) map[string]string {
	resolved := resolveControllerDefaults(gs, chartDefaults)
	filtered := make(map[string]string, len(resolved))
	for key, raw := range resolved {
		if _, skip := controllerDefaultsSkipPaths[key]; skip {
			continue
		}
		encodedSegments := strings.Split(key, ".")
		if len(encodedSegments) == 0 {
			continue
		}
		root := decodeKeySegment(encodedSegments[0])
		if _, ok := controllerDefaultsAllowedRoots[root]; !ok {
			continue
		}
		filtered[key] = raw
	}
	return filtered
}

func buildControllerDefaultTree(rawDefaults map[string]string) map[string]any {
	values := map[string]any{}
	for key, raw := range rawDefaults {
		if _, skip := controllerDefaultsSkipPaths[key]; skip {
			continue
		}
		encodedSegments := strings.Split(key, ".")
		if len(encodedSegments) == 0 {
			continue
		}
		root := decodeKeySegment(encodedSegments[0])
		if _, ok := controllerDefaultsAllowedRoots[root]; !ok {
			continue
		}
		segments := make([]string, len(encodedSegments))
		for i, seg := range encodedSegments {
			segments[i] = decodeKeySegment(seg)
		}
		setNestedValue(values, segments, formatSchemaDefault(segments, raw))
	}
	return values
}

const encodedDot = "\uff0e"

func encodeKeySegment(seg string) string {
	if seg == "" {
		return seg
	}
	return strings.ReplaceAll(seg, ".", encodedDot)
}

func decodeKeySegment(seg string) string {
	if seg == "" {
		return seg
	}
	return strings.ReplaceAll(seg, encodedDot, ".")
}

func resolveControllerDefaults(_ config.ValuesSpec, chartDefaults map[string]any) map[string]string {
	flattened := map[string]string{}
	if chartDefaults != nil {
		flattenChartDefaults(nil, chartDefaults, flattened)
	}

	resolved := make(map[string]string, len(flattened))
	for key, val := range flattened {
		resolved[key] = strings.TrimSpace(val)
	}

	return resolved
}

func flattenChartDefaults(prefix []string, value any, out map[string]string) {
	// Recurse only for nested maps. Treat everything else as a scalar.
	if m, ok := value.(map[string]any); ok {
		key := strings.Join(prefix, ".")
		if key != "" && len(m) == 0 {
			out[key] = "{}"
		}
		if len(m) == 0 {
			return
		}
		for k, child := range m {
			flattenChartDefaults(append(prefix, encodeKeySegment(k)), child, out)
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

func normalizeStringSlice(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func intMapsEqual(a, b map[string]int) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

func stringMapsEqual(a, b map[string]string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || strings.TrimSpace(bv) != strings.TrimSpace(v) {
			return false
		}
	}
	return true
}

func mapsJSONEqual(a, b map[string]any) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	aj, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bj, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(aj) == string(bj)
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
	case strings.HasSuffix(lower, "pullsecrets"), strings.HasSuffix(lower, "resourcetags"), strings.HasSuffix(lower, ".resources"), strings.HasSuffix(lower, "tolerations"), strings.Contains(lower, "extravolume"), strings.Contains(lower, "extraenv"):
		return "string[]"

	case strings.HasSuffix(lower, "labels"), strings.HasSuffix(lower, "annotations"), strings.HasSuffix(lower, "nodeselector"), strings.HasSuffix(lower, "affinity"),
		strings.HasSuffix(lower, "strategy"), strings.Contains(lower, "resourceresyncperiods"),
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
