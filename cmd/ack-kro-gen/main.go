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

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/example/ack-kro-gen/internal/config"
	"github.com/example/ack-kro-gen/internal/helmfetch"
	"github.com/example/ack-kro-gen/internal/kro"
	"github.com/example/ack-kro-gen/internal/render"
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

			sem := make(chan struct{}, flagConcurrency)
			g, ctx := errgroup.WithContext(ctx)

			for _, gspec := range cfg.Graphs {
				gs := gspec // capture
				g.Go(func() error {
					sem <- struct{}{}
					defer func() { <-sem }()

					chartRef := fmt.Sprintf("oci://public.ecr.aws/aws-controllers-k8s/%s-chart:%s", gs.Service, gs.Version)
					chartPath, err := helmfetch.EnsureChart(ctx, chartRef, flagCache, flagOffline)
					if err != nil {
						return fmt.Errorf("fetch chart for %s: %w", gs.Service, err)
					}

					r, err := render.RenderChart(ctx, chartPath, gs)
					if err != nil {
						return fmt.Errorf("render %s: %w", gs.Service, err)
					}

					// Build RGD and write
					file := filepath.Join(absOut, fmt.Sprintf("%s.rgd.yaml", gs.Service))
					if err := kro.EmitRGD(gs, r, file, absOut); err != nil {
						return fmt.Errorf("emit rgd for %s: %w", gs.Service, err)
					}
					log.Printf("wrote %s", file)
					return nil
				})
			}
			return g.Wait()
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

func max(a, b int) int { if a > b { return a }; return b }