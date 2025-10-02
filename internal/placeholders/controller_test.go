package placeholders_test

import (
	"testing"

	"github.com/jayadeyemi/ack-kro-gen/internal/config"
	"github.com/jayadeyemi/ack-kro-gen/internal/placeholders"
)

func TestControllerValuesDefaultsFromGraphSpec(t *testing.T) {
	gs := config.GraphSpec{
		Service:   "s3",
		Version:   "v0.1.0",
		Namespace: "ack-system",
		Image: config.ImageSpec{
			Repository: "public.ecr.aws/aws-controllers-k8s/s3-controller",
			Tag:        "",
		},
	}

	values := placeholders.ControllerValues(gs, nil)

	image := mustMap(t, values["image"])
	if got, want := image["tag"], "string | default=v0.1.0"; got != want {
		t.Fatalf("image.tag = %v, want %q", got, want)
	}

	leader := mustMap(t, values["leaderElection"])
	if got, want := leader["namespace"], "string | default=ack-system"; got != want {
		t.Fatalf("leaderElection.namespace = %v, want %q", got, want)
	}
}

func mustMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", v)
	}
	return m
}
