package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "graphs.yaml")
	data := `graphs:
  - service: " S3 "
    version: " 1.2.3 "
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	root, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(root.Graphs) != 1 {
		t.Fatalf("expected 1 graph, got %d", len(root.Graphs))
	}

	g := root.Graphs[0]
	if g.Service != "S3" {
		t.Fatalf("expected trimmed service, got %q", g.Service)
	}
	if g.Version != "1.2.3" {
		t.Fatalf("expected trimmed version, got %q", g.Version)
	}
	if g.Image.Tag != "1.2.3" {
		t.Fatalf("expected tag default to version, got %q", g.Image.Tag)
	}
	if g.ReleaseName != "ack-s3-controller" {
		t.Fatalf("expected default release name, got %q", g.ReleaseName)
	}
	if g.Namespace != "ack-system" {
		t.Fatalf("expected default namespace, got %q", g.Namespace)
	}
}
