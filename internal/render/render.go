// Package render loads a cached Helm chart archive, renders its templates with computed values,
// and returns two buckets:
//   1) RenderedFiles: rendered YAML from templates/ (root + subcharts), keyed by file path.
//   2) CRDs:          raw YAML from crds/ (root + subcharts), returned verbatim without templating.
//
// Key points:
// - CRDs are read via ch.CRDObjects() and are NOT templated. Helm installs CRDs first.
// - Controller manifests are produced by Helm's template engine from templates/ using computed values.
// - Output file order is made deterministic for stable diffs/tests.

package render

import (
	"context"
	"fmt"

	// "maps" // optional in newer Go for map utilities
	"path/filepath"
	"sort"
	"strings"

	"github.com/jayadeyemi/ack-kro-gen/internal/config"
	"github.com/jayadeyemi/ack-kro-gen/internal/placeholders"

	// "gopkg.in/yaml.v3" // not needed here
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
)

// Result groups the two output sets from a chart render.
type Result struct {
	// RenderedFiles holds rendered YAML from templates/. Each value may be a multi-document YAML string.
	// Key is the normalized template path (using forward slashes).
	RenderedFiles map[string]string
	// CRDs contains raw YAML documents from crds/ files. No templating is applied.
	CRDs []string
	// ChartValues captures the chart's default values from values.yaml for downstream defaults processing.
	ChartValues map[string]any
}

// RenderChart loads a Helm chart archive (or directory), renders templates with values derived from
// the provided ValuesSpec, and splits outputs into CRDs and controller manifests.
func RenderChart(ctx context.Context, chartArchivePath string, gs config.ValuesSpec) (*Result, error) {
	// Load the chart from a .tgz path or directory. No network access here.
	ch, err := loader.Load(chartArchivePath)
	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}

	chartDefaults := cloneMap(ch.Values)

	// Build the values map to feed into Helm's renderer based on ValuesSpec.
	vals := buildValues(gs)

	// Emulate a Helm release for templating. These can be used by templates as .Release.*.
	rel := chartutil.ReleaseOptions{
		Name:      "_NAME_",          // placeholder; not persisted to outputs
		Namespace: "_KRO_NAMESPACE_", // placeholder; not persisted to outputs
		IsInstall: true,
		Revision:  1,
	}

	// Capabilities influence templates that gate on Kubernetes version or APIs.
	caps := chartutil.DefaultCapabilities
	caps.KubeVersion.Version = "v1.27.0"

	// Merge chart defaults, user values, release, and capabilities into a render-ready structure.
	rvals, err := chartutil.ToRenderValues(ch, vals, rel, caps)
	if err != nil {
		return nil, fmt.Errorf("render values: %w", err)
	}

	// Render all templates from templates/ across root chart and subcharts.
	// Returns a map[path]renderedText for every template file, regardless of extension.
	renderer := engine.Engine{}
	files, err := renderer.Render(ch, rvals.AsMap())
	if err != nil {
		return nil, fmt.Errorf("engine render: %w", err)
	}

	// Collect CRDs from crds/ directories. These are emitted verbatim and not templated by Helm.
	runtimeSentinels := placeholders.BuildRuntimeSentinels(gs)

	var crds []string
	for _, obj := range ch.CRDObjects() {
		body := string(obj.File.Data)
		body = placeholders.ApplyRuntimeSentinels(body, runtimeSentinels)
		crds = append(crds, body)
	}

	// Keep only YAML artifacts from the rendered templates. Drop non-YAML files (helpers, txt, etc).
	out := map[string]string{}
	for name, body := range files {
		// Only accept .yaml/.yml. Some charts emit NOTES.txt or other non-manifest outputs.
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		// Normalize path separators for deterministic keys on all OSes.
		// Also trim and ensure a single trailing newline for clean diffs.
		body = placeholders.ApplyRuntimeSentinels(body, runtimeSentinels)
		out[filepath.ToSlash(name)] = strings.TrimSpace(body) + "\n"
	}

	// Deterministic iteration order: sort keys and rebuild a new map.
	// This avoids nondeterministic map iteration in tests and downstream processing.
	ordered := map[string]string{}
	keys := make([]string, 0, len(out))
	for k := range out {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		ordered[k] = out[k]
	}

	// Return controller manifests (ordered) and raw CRDs.
	return &Result{RenderedFiles: ordered, CRDs: crds, ChartValues: chartDefaults}, nil
}

// buildValues constructs the Helm values map consumed by the ACK controller chart
// using only the ValuesSpec input. Callers layer any additional overrides before
// invoking Helm.
func buildValues(gs config.ValuesSpec) map[string]any {
	values := map[string]any{}

	// Image settings.
	image := map[string]any{}
	if repo := strings.TrimSpace(gs.Image.Repository); repo != "" {
		image["repository"] = repo
	}
	tag := strings.TrimSpace(gs.Image.Tag)
	if tag == "" {
		tag = strings.TrimSpace(gs.Version)
	}
	if tag != "" {
		image["tag"] = tag
	}
	if policy := strings.TrimSpace(gs.Image.PullPolicy); policy != "" {
		image["pullPolicy"] = policy
	}
	if len(gs.Image.PullSecrets) > 0 {
		secrets := make([]string, 0, len(gs.Image.PullSecrets))
		for _, s := range gs.Image.PullSecrets {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				secrets = append(secrets, trimmed)
			}
		}
		if len(secrets) > 0 {
			image["pullSecrets"] = secrets
		}
	}
	if len(image) > 0 {
		values["image"] = image
	}

	// Chart name overrides.
	if v := strings.TrimSpace(gs.NameOverride); v != "" {
		values["nameOverride"] = v
	}
	if v := strings.TrimSpace(gs.FullnameOverride); v != "" {
		values["fullnameOverride"] = v
	}

	// ServiceAccount configuration.
	sa := map[string]any{}
	if !gs.ServiceAccount.Create {
		sa["create"] = false
	}
	if name := strings.TrimSpace(gs.ServiceAccount.Name); name != "" {
		sa["name"] = name
	}
	if len(gs.ServiceAccount.Annotations) > 0 {
		ann := make(map[string]any, len(gs.ServiceAccount.Annotations))
		for k, v := range gs.ServiceAccount.Annotations {
			ann[k] = v
		}
		sa["annotations"] = ann
	}
	if len(sa) > 0 {
		values["serviceAccount"] = sa
	}

	// Logging configuration.
	logVals := map[string]any{}
	if level := strings.TrimSpace(gs.Log.Level); level != "" {
		logVals["level"] = level
	}
	if gs.Log.EnableDevelopmentLogging {
		logVals["enable_development_logging"] = true
	}
	if len(logVals) > 0 {
		values["log"] = logVals
	}

	// Deployment namespace scoping.
	if watch := strings.TrimSpace(gs.WatchNamespace); watch != "" {
		values["watchNamespace"] = watch
	}
	if selectors := strings.TrimSpace(gs.WatchSelectors); selectors != "" {
		values["watchSelectors"] = selectors
	}
	if scope := strings.TrimSpace(gs.InstallScope); scope != "" {
		values["installScope"] = scope
	}

	// AWS configuration (region, endpoint, credentials).
	awsVals := map[string]any{}
	if region := strings.TrimSpace(gs.AWS.Region); region != "" {
		awsVals["region"] = region
	}
	if endpoint := strings.TrimSpace(gs.AWS.EndpointURL); endpoint != "" {
		awsVals["endpoint_url"] = endpoint
	}
	credVals := map[string]any{}
	if secretName := strings.TrimSpace(gs.AWS.Credentials.SecretName); secretName != "" {
		credVals["secretName"] = secretName
	}
	if secretKey := strings.TrimSpace(gs.AWS.Credentials.SecretKey); secretKey != "" {
		credVals["secretKey"] = secretKey
	}
	if profile := strings.TrimSpace(gs.AWS.Credentials.Profile); profile != "" {
		credVals["profile"] = profile
	}
	if len(credVals) > 0 {
		awsVals["credentials"] = credVals
	}
	if len(awsVals) > 0 {
		values["aws"] = awsVals
	}

	// Metrics service configuration.
	metrics := map[string]any{}
	serviceMetrics := map[string]any{}
	if gs.Metrics.Service.Create {
		serviceMetrics["create"] = true
	}
	if svcType := strings.TrimSpace(gs.Metrics.Service.Type); svcType != "" {
		serviceMetrics["type"] = svcType
	}
	if len(serviceMetrics) > 0 {
		metrics["service"] = serviceMetrics
	}
	if len(metrics) > 0 {
		values["metrics"] = metrics
	}

	// Resources (requests/limits).
	resourceVals := map[string]any{}
	if len(gs.Resources.Requests) > 0 {
		reqs := make(map[string]any, len(gs.Resources.Requests))
		for k, v := range gs.Resources.Requests {
			reqs[k] = strings.TrimSpace(v)
		}
		resourceVals["requests"] = reqs
	}
	if len(gs.Resources.Limits) > 0 {
		limits := make(map[string]any, len(gs.Resources.Limits))
		for k, v := range gs.Resources.Limits {
			limits[k] = strings.TrimSpace(v)
		}
		resourceVals["limits"] = limits
	}
	if len(resourceVals) > 0 {
		values["resources"] = resourceVals
	}

	// Role labels.
	if len(gs.Role.Labels) > 0 {
		labels := make(map[string]any, len(gs.Role.Labels))
		for k, v := range gs.Role.Labels {
			labels[k] = v
		}
		values["role"] = map[string]any{"labels": labels}
	}

	// Deployment tuning.
	deployment := map[string]any{}
	if len(gs.Deployment.Annotations) > 0 {
		ann := make(map[string]any, len(gs.Deployment.Annotations))
		for k, v := range gs.Deployment.Annotations {
			ann[k] = v
		}
		deployment["annotations"] = ann
	}
	if len(gs.Deployment.Labels) > 0 {
		lbl := make(map[string]any, len(gs.Deployment.Labels))
		for k, v := range gs.Deployment.Labels {
			lbl[k] = v
		}
		deployment["labels"] = lbl
	}
	if port := gs.Deployment.ContainerPort; port > 0 {
		deployment["containerPort"] = port
	}
	if replicas := gs.Deployment.Replicas; replicas > 0 {
		deployment["replicas"] = replicas
	}
	if len(gs.Deployment.NodeSelector) > 0 {
		ns := make(map[string]any, len(gs.Deployment.NodeSelector))
		for k, v := range gs.Deployment.NodeSelector {
			ns[k] = v
		}
		deployment["nodeSelector"] = ns
	}
	if len(gs.Deployment.Tolerations) > 0 {
		deployment["tolerations"] = cloneSliceOfMaps(gs.Deployment.Tolerations)
	}
	if len(gs.Deployment.Affinity) > 0 {
		deployment["affinity"] = cloneMap(gs.Deployment.Affinity)
	}
	if pc := strings.TrimSpace(gs.Deployment.PriorityClassName); pc != "" {
		deployment["priorityClassName"] = pc
	}
	if gs.Deployment.HostNetwork {
		deployment["hostNetwork"] = true
	}
	if dns := strings.TrimSpace(gs.Deployment.DNSPolicy); dns != "" {
		deployment["dnsPolicy"] = dns
	}
	if len(gs.Deployment.Strategy) > 0 {
		deployment["strategy"] = cloneMap(gs.Deployment.Strategy)
	}
	if len(gs.Deployment.ExtraVolumes) > 0 {
		deployment["extraVolumes"] = cloneSliceOfMaps(gs.Deployment.ExtraVolumes)
	}
	if len(gs.Deployment.ExtraVolumeMounts) > 0 {
		deployment["extraVolumeMounts"] = cloneSliceOfMaps(gs.Deployment.ExtraVolumeMounts)
	}
	if len(gs.Deployment.ExtraEnvVars) > 0 {
		deployment["extraEnvVars"] = cloneSliceOfMaps(gs.Deployment.ExtraEnvVars)
	}
	if len(deployment) > 0 {
		values["deployment"] = deployment
	}

	// Reconcile tuning.
	reconcile := map[string]any{}
	if period := gs.Reconcile.DefaultResyncPeriod; period > 0 {
		reconcile["defaultResyncPeriod"] = period
	}
	if max := gs.Reconcile.DefaultMaxConcurrentSyncs; max > 0 {
		reconcile["defaultMaxConcurrentSyncs"] = max
	}
	if len(gs.Reconcile.ResourceResyncPeriods) > 0 {
		periods := make(map[string]any, len(gs.Reconcile.ResourceResyncPeriods))
		for k, v := range gs.Reconcile.ResourceResyncPeriods {
			periods[k] = v
		}
		reconcile["resourceResyncPeriods"] = periods
	}
	if len(gs.Reconcile.ResourceMaxConcurrentSyncs) > 0 {
		maxes := make(map[string]any, len(gs.Reconcile.ResourceMaxConcurrentSyncs))
		for k, v := range gs.Reconcile.ResourceMaxConcurrentSyncs {
			maxes[k] = v
		}
		reconcile["resourceMaxConcurrentSyncs"] = maxes
	}
	if len(gs.Reconcile.Resources) > 0 {
		reconcile["resources"] = append([]string{}, gs.Reconcile.Resources...)
	}
	if len(reconcile) > 0 {
		values["reconcile"] = reconcile
	}

	// Leader election.
	le := map[string]any{}
	if gs.LeaderElection.Enabled {
		le["enabled"] = true
	}
	if ns := strings.TrimSpace(gs.LeaderElection.Namespace); ns != "" {
		le["namespace"] = ns
	}
	if len(le) > 0 {
		values["leaderElection"] = le
	}

	// Global flags.
	if len(gs.ResourceTags) > 0 {
		tags := make([]string, 0, len(gs.ResourceTags))
		for _, tag := range gs.ResourceTags {
			if trimmed := strings.TrimSpace(tag); trimmed != "" {
				tags = append(tags, trimmed)
			}
		}
		if len(tags) > 0 {
			values["resourceTags"] = tags
		}
	}
	if policy := strings.TrimSpace(gs.DeletionPolicy); policy != "" {
		values["deletionPolicy"] = policy
	}
	if !gs.EnableCARM {
		values["enableCARM"] = false
	}

	featureGateDefaults := map[string]bool{
		"ServiceLevelCARM":  false,
		"TeamLevelCARM":     false,
		"ReadOnlyResources": true,
		"ResourceAdoption":  true,
	}
	featureGates := map[string]any{}
	if gs.FeatureGates.ServiceLevelCARM != featureGateDefaults["ServiceLevelCARM"] {
		featureGates["ServiceLevelCARM"] = gs.FeatureGates.ServiceLevelCARM
	}
	if gs.FeatureGates.TeamLevelCARM != featureGateDefaults["TeamLevelCARM"] {
		featureGates["TeamLevelCARM"] = gs.FeatureGates.TeamLevelCARM
	}
	if gs.FeatureGates.ReadOnlyResources != featureGateDefaults["ReadOnlyResources"] {
		featureGates["ReadOnlyResources"] = gs.FeatureGates.ReadOnlyResources
	}
	if gs.FeatureGates.ResourceAdoption != featureGateDefaults["ResourceAdoption"] {
		featureGates["ResourceAdoption"] = gs.FeatureGates.ResourceAdoption
	}
	if len(featureGates) > 0 {
		values["featureGates"] = featureGates
	}

	return values
}

// deepMerge recursively merges src into dst. For map values, it recurses. For scalars, src overwrites dst.
// Assumes both dst and src are map[string]any.
func deepMerge(dst, src map[string]any) {
	for k, v := range src {
		if vmap, ok := v.(map[string]any); ok {
			// src value is a map
			if dmap, ok := dst[k].(map[string]any); ok {
				// both are maps → recurse
				deepMerge(dmap, vmap)
				dst[k] = dmap
			} else {
				// dst is missing or not a map → clone src map to avoid aliasing
				dst[k] = cloneMap(vmap)
			}
		} else {
			// scalar or non-map → overwrite
			dst[k] = v
		}
	}
}

// cloneMap produces a deep copy of a map[string]any, cloning nested maps recursively.
// Used to avoid sharing references during merges.
func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if mv, ok := v.(map[string]any); ok {
			out[k] = cloneMap(mv)
		} else {
			out[k] = v
		}
	}
	return out
}

func cloneSliceOfMaps(list []map[string]any) []map[string]any {
	clone := make([]map[string]any, len(list))
	for i, item := range list {
		clone[i] = cloneMap(item)
	}
	return clone
}

// SplitYAML is re-exported for tests and callers that need to split multi-doc YAML strings.
// It delegates to internal/render.SplitYAML.
func SplitYAML(s string) []string {
	parts := []string{}
	for _, p := range strings.Split(s, "\n---") {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		// Ensure trailing newline for deterministic encoding later
		if !strings.HasSuffix(t, "\n") {
			t += "\n"
		}
		parts = append(parts, t)
	}
	return parts
}
