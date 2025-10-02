package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jayadeyemi/ack-kro-gen/internal/config"
)

func TestLoadDefaultsWhenOptionalFieldsMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "graphs.yaml")
	yaml := "graphs:\n  - service: \" s3 \"\n    version: \" v0.1.0 \"\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	root, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(root.Graphs) != 1 {
		t.Fatalf("expected 1 graph, got %d", len(root.Graphs))
	}

	g := root.Graphs[0]
	if g.Service != "s3" {
		t.Fatalf("expected service to be trimmed 's3', got %q", g.Service)
	}
	if g.Version != "v0.1.0" {
		t.Fatalf("expected version to be trimmed 'v0.1.0', got %q", g.Version)
	}
	if g.Image.Tag != "v0.1.0" {
		t.Fatalf("expected image tag to default to version, got %q", g.Image.Tag)
	}
	if g.ReleaseName != "ack-s3-controller" {
		t.Fatalf("expected releaseName default, got %q", g.ReleaseName)
	}
	if g.Namespace != "ack-system" {
		t.Fatalf("expected namespace default, got %q", g.Namespace)
	}
}
