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
	"github.com/jayadeyemi/ack-kro-gen/internal/util"
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
		Name:      "__KRO_NAME__", // placeholder; not persisted to outputs
		Namespace: "__KRO_NS__",   // placeholder; not persisted to outputs
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
//   1) <service>-chart (e.g., "s3-chart")
//   2) "ack-chart" (shared defaults across ACK controllers)
//   3) any other top-level keys, merged into the base under their own key
// Base seeds image, serviceAccount, log flags, and AWS region.
func buildValues(gs config.GraphSpec) map[string]any {
	// Base values seeded from GraphSpec.
	base := map[string]any{
		"image": map[string]any{
			"repository": gs.Image.Repository,
			"tag":        gs.Image.Tag,
		},
		"serviceAccount": map[string]any{
			"name":        gs.ServiceAccount.Name, // string name
			"annotations": map[string]any{},      // coerced to map[string]any below if needed
		},
		"logLevel": gs.Controller.LogLevel,
		"logDev":   gs.Controller.LogDev,
		"aws": map[string]any{
			"region": gs.AWS.Region,
		},
	}

	// If annotations are provided in GraphSpec, copy them into the base map.
	if len(gs.ServiceAccount.Annotations) > 0 {
		ann := make(map[string]any, len(gs.ServiceAccount.Annotations))
		for k, v := range gs.ServiceAccount.Annotations {
			ann[k] = v
		}
		base["serviceAccount"].(map[string]any)["annotations"] = ann
	}

	// Merge user-provided values by precedence.
	if gs.Extras.Values != nil {
		svcKey := gs.Service + "-chart"

		// 1) Service-specific overlay wins first.
		if v, ok := gs.Extras.Values[svcKey]; ok {
			if mv, ok := v.(map[string]any); ok {
				deepMerge(base, mv)
			}
		}
		// 2) Shared "ack-chart" overlay wins next.
		if v, ok := gs.Extras.Values["ack-chart"]; ok {
			if mv, ok := v.(map[string]any); ok {
				deepMerge(base, mv)
			}
		}
		// 3) Remaining keys are merged into or assigned at their respective top-level keys.
		for k, v := range gs.Extras.Values {
			if k == svcKey || k == "ack-chart" {
				continue
			}
			if mv, ok := v.(map[string]any); ok {
				// If base already has a map for this key, merge into it. Otherwise clone mv into base.
				if cur, ok := base[k].(map[string]any); ok {
					deepMerge(cur, mv)
					base[k] = cur
				} else {
					base[k] = cloneMap(mv)
				}
			} else {
				// Scalars overwrite directly.
				base[k] = v
			}
		}
	}

	// Normalize serviceAccount type:
	// If a chart expects serviceAccount to be a map, but a string name is supplied,
	// convert the string form into a structured map with create:false and empty annotations.
	if name, ok := base["serviceAccount"].(string); ok {
		base["serviceAccount"] = map[string]any{
			"create":      false,
			"name":        name,
			"annotations": map[string]any{},
		}
	}

	// Ensure serviceAccount is a map and annotations is map[string]any.
	sa, ok := base["serviceAccount"].(map[string]any)
	if !ok {
		sa = map[string]any{"name": gs.ServiceAccount.Name}
	}
	base["serviceAccount"] = sa

	switch a := sa["annotations"].(type) {
	case map[string]any:
		// already correct
	case map[string]string:
		// copy into map[string]any for consistent types in templates.
		m := make(map[string]any, len(a))
		for k, v := range a {
			m[k] = v
		}
		sa["annotations"] = m
	default:
		// ensure non-nil map
		sa["annotations"] = map[string]any{}
	}

	return base
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
// It delegates to internal/util.SplitYAML.
func SplitYAML(s string) []string { return util.SplitYAML(s) }
