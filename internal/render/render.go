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
	"strconv"
	"strings"

	"github.com/jayadeyemi/ack-kro-gen/internal/config"

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
}

// RenderChart loads a Helm chart archive (or directory), renders templates with values derived from
// the provided GraphSpec, and splits outputs into CRDs and controller manifests.
func RenderChart(ctx context.Context, chartArchivePath string, gs config.GraphSpec) (*Result, error) {
	// Load the chart from a .tgz path or directory. No network access here.
	ch, err := loader.Load(chartArchivePath)
	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}

	// Build the values map to feed into Helm's renderer based on GraphSpec.
	vals := buildValues(gs)

	// Emulate a Helm release for templating. These can be used by templates as .Release.*.
	rel := chartutil.ReleaseOptions{
		Name:      "__KRO_NAME__",      // placeholder; not persisted to outputs
		Namespace: "__KRO_NAMESPACE__", // placeholder; not persisted to outputs
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
	var crds []string
	for _, obj := range ch.CRDObjects() {
		crds = append(crds, string(obj.File.Data))
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
	return &Result{RenderedFiles: ordered, CRDs: crds}, nil
}

// buildValues constructs the Helm values map from GraphSpec, then merges in optional overrides.
// Precedence (highest to lower within Extras.Values):
//  1. <service>-chart (e.g., "s3-chart")
//  2. "ack-chart" (shared defaults across ACK controllers)
//  3. any other top-level keys, merged into the base under their own key
//
// Base seeds image, serviceAccount, log flags, and AWS region.
func buildValues(gs config.GraphSpec) map[string]any {
	values := map[string]any{}

	repo := strings.TrimSpace(gs.Image.Repository)
	tag := strings.TrimSpace(gs.Image.Tag)
	if tag == "" {
		tag = strings.TrimSpace(gs.Version)
	}

	if repo != "" || tag != "" {
		image := map[string]any{}
		if repo != "" {
			image["repository"] = repo
		}
		if tag != "" {
			image["tag"] = tag
		}
		values["image"] = image
	}

	if name := strings.TrimSpace(gs.ServiceAccount.Name); name != "" || len(gs.ServiceAccount.Annotations) > 0 {
		sa := map[string]any{}
		if name != "" {
			sa["name"] = name
		}
		if len(gs.ServiceAccount.Annotations) > 0 {
			ann := make(map[string]any, len(gs.ServiceAccount.Annotations))
			for k, v := range gs.ServiceAccount.Annotations {
				ann[k] = v
			}
			sa["annotations"] = ann
		}
		values["serviceAccount"] = sa
	}

	logLevel := strings.TrimSpace(gs.Controller.LogLevel)
	logDev := strings.TrimSpace(gs.Controller.LogDev)
	if logLevel != "" || logDev != "" {
		logVals := map[string]any{}
		if logLevel != "" {
			logVals["level"] = logLevel
		}
		if logDev != "" {
			if b, err := strconv.ParseBool(logDev); err == nil {
				logVals["enable_development_logging"] = b
			}
		}
		if len(logVals) > 0 {
			values["log"] = logVals
		}
	}

	if watch := strings.TrimSpace(gs.Controller.WatchNamespace); watch != "" {
		values["watchNamespace"] = watch
	}

	awsVals := map[string]any{}
	if region := strings.TrimSpace(gs.AWS.Region); region != "" {
		awsVals["region"] = region
	}
	credVals := map[string]any{}
	if secretName := strings.TrimSpace(gs.AWS.SecretName); secretName != "" {
		credVals["secretName"] = secretName
	}
	if secretKey := strings.TrimSpace(gs.AWS.Credentials); secretKey != "" {
		credVals["secretKey"] = secretKey
	}
	if profile := strings.TrimSpace(gs.AWS.Profile); profile != "" {
		credVals["profile"] = profile
	}
	if len(credVals) > 0 {
		awsVals["credentials"] = credVals
	}
	if len(awsVals) > 0 {
		values["aws"] = awsVals
	}

	// Merge user-provided values by precedence.
	if gs.Extras.Values != nil {
		svcKey := gs.Service + "-chart"

		// 1) Service-specific overlay wins first.
		if v, ok := gs.Extras.Values[svcKey]; ok {
			if mv, ok := v.(map[string]any); ok {
				deepMerge(values, mv)
			}
		}
		// 2) Shared "ack-chart" overlay wins next.
		if v, ok := gs.Extras.Values["ack-chart"]; ok {
			if mv, ok := v.(map[string]any); ok {
				deepMerge(values, mv)
			}
		}
		// 3) Remaining keys are merged into or assigned at their respective top-level keys.
		for k, v := range gs.Extras.Values {
			if k == svcKey || k == "ack-chart" {
				continue
			}
			if mv, ok := v.(map[string]any); ok {
				// If values already has a map for this key, merge into it. Otherwise clone mv into values.
				if cur, ok := values[k].(map[string]any); ok {
					deepMerge(cur, mv)
					values[k] = cur
				} else {
					values[k] = cloneMap(mv)
				}
			} else {
				// Scalars overwrite directly.
				values[k] = v
			}
		}
	}

	// Normalize serviceAccount type when passed as a string via overrides.
	if name, ok := values["serviceAccount"].(string); ok {
		values["serviceAccount"] = map[string]any{
			"create":      false,
			"name":        name,
			"annotations": map[string]any{},
		}
	}

	if sa, ok := values["serviceAccount"].(map[string]any); ok {
		switch a := sa["annotations"].(type) {
		case map[string]any:
			// already normalized
		case map[string]string:
			m := make(map[string]any, len(a))
			for k, v := range a {
				m[k] = v
			}
			sa["annotations"] = m
		case nil:
			sa["annotations"] = map[string]any{}
		default:
			sa["annotations"] = map[string]any{}
		}
		values["serviceAccount"] = sa
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
