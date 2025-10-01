package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/jayadeyemi/ack-kro-gen/internal/config"
	"github.com/jayadeyemi/ack-kro-gen/internal/helmfetch"
	"github.com/jayadeyemi/ack-kro-gen/internal/kro"
	"github.com/jayadeyemi/ack-kro-gen/internal/render"
	"github.com/jayadeyemi/ack-kro-gen/internal/util"
)

var (
	flagGraphs      string
	flagOut         string
	flagCache       string
	flagOffline     bool
	flagConcurrency int
	flagLogLevel    string
)

func main() {
	root := &cobra.Command{
		Use:   "ack-kro-gen",
		Short: "Generate KRO RGDs for AWS ACK controllers",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagGraphs == "" || flagOut == "" || flagCache == "" {
				return errors.New("--graphs, --out, and --charts-cache are required")
			}

			log.Printf("start: graphs=%s out=%s cache=%s offline=%v concurrency=%d", flagGraphs, flagOut, flagCache, flagOffline, flagConcurrency)

			absOut, err := filepath.Abs(flagOut)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(absOut, 0o755); err != nil {
				return fmt.Errorf("create out dir: %w", err)
			}

			cfg, err := config.Load(flagGraphs)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
			defer cancel()

			start := time.Now()
			sem := make(chan struct{}, flagConcurrency)
			g, ctx := errgroup.WithContext(ctx)

			for _, gspec := range cfg.Graphs {
				gs := gspec // capture
				g.Go(func() error {
					sem <- struct{}{}
					defer func() { <-sem }()

					chartRef := fmt.Sprintf("oci://public.ecr.aws/aws-controllers-k8s/%s-chart:%s", gs.Service, gs.Version)
					log.Printf("[%s] fetch: ref=%s", gs.Service, chartRef)
					chartPath, err := helmfetch.EnsureChart(ctx, chartRef, flagCache, flagOffline)
					if err != nil {
						return fmt.Errorf("fetch chart for %s: %w", gs.Service, err)
					}
					log.Printf("[%s] fetch: cached at %s", gs.Service, chartPath)

					log.Printf("[%s] render: begin", gs.Service)
					r, err := render.RenderChart(ctx, chartPath, gs)
					if err != nil {
						return fmt.Errorf("render %s: %w", gs.Service, err)
					}
					log.Printf("[%s] render: crds=%d files=%d", gs.Service, len(r.CRDs), len(r.RenderedFiles))

					// Quick preview of first few manifest doc kinds for visibility
					firstKinds := []string{}
					maxPreview := 5
					for _, body := range r.RenderedFiles {
						for _, doc := range util.SplitYAML(body) {
							if len(firstKinds) >= maxPreview {
								break
							}
							var k struct {
								Kind string `yaml:"kind"`
							}
							if err := yaml.Unmarshal([]byte(doc), &k); err == nil && k.Kind != "" {
								firstKinds = append(firstKinds, k.Kind)
							}
						}
						if len(firstKinds) >= maxPreview {
							break
						}
					}
					log.Printf("[%s] render: preview kinds=%v", gs.Service, firstKinds)

					// Write separate CRD and controller graphs named from their RGD metadata.name
					log.Printf("[%s] emit: begin", gs.Service)
					wrote, err := kro.EmitRGDs(gs, r, absOut)
					if err != nil {
						return fmt.Errorf("emit rgds for %s: %w", gs.Service, err)
					}
					for _, f := range wrote {
						fi, _ := os.Stat(f)
						size := int64(-1)
						if fi != nil {
							size = fi.Size()
						}
						log.Printf("[%s] emit: wrote %s bytes=%d", gs.Service, f, size)
					}
					log.Printf("[%s] done", gs.Service)
					return nil
				})
			}
			if err := g.Wait(); err != nil {
				return err
			}
			log.Printf("complete in %s", time.Since(start).Round(time.Millisecond))
			return nil
		},
	}

	root.Flags().StringVar(&flagGraphs, "graphs", "graphs.yaml", "graphs.yaml path")
	root.Flags().StringVar(&flagOut, "out", "out", "output directory")
	root.Flags().StringVar(&flagCache, "charts-cache", ".cache/charts", "local chart cache directory")
	root.Flags().BoolVar(&flagOffline, "offline", false, "offline mode, read charts only from cache")
	root.Flags().IntVar(&flagConcurrency, "concurrency", max(2, runtime.NumCPU()), "parallel services")
	root.Flags().StringVar(&flagLogLevel, "log-level", "info", "log level: info|debug")

	if err := root.Execute(); err != nil {
		if !strings.HasSuffix(err.Error(), "help requested") {
			log.Fatal(err)
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
