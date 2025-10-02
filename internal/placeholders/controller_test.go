package placeholders

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jayadeyemi/ack-kro-gen/internal/config"
)

func TestControllerValuesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "graphs.yaml")
	data := `graphs:
  - service: s3
    version: 1.2.3
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	root, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	gs := root.Graphs[0]

	values := ControllerValues(gs, nil)

	image, ok := values["image"].(map[string]any)
	if !ok {
		t.Fatalf("expected image map in controller values")
	}
	if got := image["tag"]; got != "string | default=1.2.3" {
		t.Fatalf("expected tag default placeholder, got %v", got)
	}

	leader, ok := values["leaderElection"].(map[string]any)
	if !ok {
		t.Fatalf("expected leaderElection map in controller values")
	}
	if got := leader["namespace"]; got != "string | default=ack-system" {
		t.Fatalf("expected namespace default placeholder, got %v", got)
	}
}
