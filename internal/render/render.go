package render

import (
	"context"
	"fmt"
	// "maps"
	"path/filepath"
	"sort"
	"strings"

	"github.com/example/ack-kro-gen/internal/config"
	"github.com/example/ack-kro-gen/internal/util"
	// "gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
)

type Result struct {
	RenderedFiles map[string]string // filename -> YAML text (multi-doc allowed)
	CRDs          []string          // YAML docs for CRDs
}

func RenderChart(ctx context.Context, chartArchivePath string, gs config.GraphSpec) (*Result, error) {
	ch, err := loader.Load(chartArchivePath)
	if err != nil { return nil, fmt.Errorf("load chart: %w", err) }

	vals := buildValues(gs)
	rel := chartutil.ReleaseOptions{
		Name:      "__KRO_NAME__",
		Namespace: "__KRO_NS__",
		IsInstall: true,
		Revision:  1,
	}
	caps := chartutil.DefaultCapabilities
	caps.KubeVersion.Version = "v1.27.0"

	rvals, err := chartutil.ToRenderValues(ch, vals, rel, caps)
	if err != nil { return nil, fmt.Errorf("render values: %w", err) }

	renderer := engine.Engine{}
	files, err := renderer.Render(ch, rvals.AsMap())
	if err != nil { return nil, fmt.Errorf("engine render: %w", err) }

	// Collect CRDs separately for ordering.
	var crds []string
	for _, obj := range ch.CRDObjects() {
		crds = append(crds, string(obj.File.Data))
	}

	// Keep only YAML manifests from templates
	out := map[string]string{}
	for name, body := range files {
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") { continue }
		// Normalize path for determinism
		out[filepath.ToSlash(name)] = strings.TrimSpace(body) + "\n"
	}

	// Deterministic file order not strictly needed since we classify later. Sort keys so tests stable.
	ordered := map[string]string{}
	keys := make([]string, 0, len(out))
	for k := range out { keys = append(keys, k) }
	sort.Strings(keys)
	for _, k := range keys { ordered[k] = out[k] }

	return &Result{RenderedFiles: ordered, CRDs: crds}, nil
}

func buildValues(gs config.GraphSpec) map[string]any {
  base := map[string]any{
    "image": map[string]any{
      "repository": gs.Image.Repository,
      "tag":        gs.Image.Tag,
    },
    "serviceAccount": map[string]any{
      "name":        gs.ServiceAccount.Name,
      "annotations": map[string]any{},
    },
    "logLevel": gs.Controller.LogLevel,
    "logDev":   gs.Controller.LogDev,
    "aws": map[string]any{
      "region": gs.Controller.AWSRegion,
    },
  }

  if len(gs.ServiceAccount.Annotations) > 0 {
    ann := make(map[string]any, len(gs.ServiceAccount.Annotations))
    for k, v := range gs.ServiceAccount.Annotations {
      ann[k] = v
    }
    base["serviceAccount"].(map[string]any)["annotations"] = ann
  }

  if gs.Extras.Values != nil {
    svcKey := gs.Service + "-chart"
    if v, ok := gs.Extras.Values[svcKey]; ok {
      if mv, ok := v.(map[string]any); ok {
        deepMerge(base, mv)
      }
    }
    if v, ok := gs.Extras.Values["ack-chart"]; ok {
      if mv, ok := v.(map[string]any); ok {
        deepMerge(base, mv)
      }
    }
    for k, v := range gs.Extras.Values {
      if k == svcKey || k == "ack-chart" {
        continue
      }
      if mv, ok := v.(map[string]any); ok {
        if cur, ok := base[k].(map[string]any); ok {
          deepMerge(cur, mv)
          base[k] = cur
        } else {
          base[k] = cloneMap(mv)
        }
      } else {
        base[k] = v
      }
    }
  }

  if name, ok := base["serviceAccount"].(string); ok {
    base["serviceAccount"] = map[string]any{
      "create":      false,
      "name":        name,
      "annotations": map[string]any{},
    }
  }
  sa, ok := base["serviceAccount"].(map[string]any)
  if !ok {
    sa = map[string]any{"name": gs.ServiceAccount.Name}
  }
  base["serviceAccount"] = sa

  switch a := sa["annotations"].(type) {
  case map[string]any:
  case map[string]string:
    m := make(map[string]any, len(a))
    for k, v := range a {
      m[k] = v
    }
    sa["annotations"] = m
  default:
    sa["annotations"] = map[string]any{}
  }

  return base
}

func deepMerge(dst, src map[string]any) {
  for k, v := range src {
    if vmap, ok := v.(map[string]any); ok {
      if dmap, ok := dst[k].(map[string]any); ok {
        deepMerge(dmap, vmap)
        dst[k] = dmap
      } else {
        dst[k] = cloneMap(vmap)
      }
    } else {
      dst[k] = v
    }
  }
}

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


// SplitYAML converts multi-doc strings into individual docs. Re-exported for tests.
func SplitYAML(s string) []string { return util.SplitYAML(s) }