package render

import (
	"testing"

	"github.com/jayadeyemi/ack-kro-gen/internal/config"
)

func TestBuildValuesDefaultsImageTagFromVersion(t *testing.T) {
	gs := config.GraphSpec{
		Version: "v1.2.3",
		Image: config.ImageSpec{
			Repository: "public.ecr.aws/aws-controllers-k8s/s3-controller",
			Tag:        "  ",
		},
	}

	values := buildValues(gs)
	image, ok := values["image"].(map[string]any)
	if !ok {
		t.Fatalf("image values missing or wrong type: %#v", values["image"])
	}
	if got, want := image["tag"], "v1.2.3"; got != want {
		t.Fatalf("image.tag = %v, want %q", got, want)
	}
}
