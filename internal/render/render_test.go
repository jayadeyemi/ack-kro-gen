package render

import (
	"testing"

	"github.com/jayadeyemi/ack-kro-gen/internal/config"
)

func TestBuildValuesDefaultsTag(t *testing.T) {
	gs := config.GraphSpec{
		Service: "s3",
		Version: "1.2.3",
		Image: config.ImageSpec{
			Repository: "public.ecr.aws/aws-controllers-k8s/s3-controller",
			Tag:        "   ",
		},
	}

	vals := buildValues(gs)
	image, ok := vals["image"].(map[string]any)
	if !ok {
		t.Fatalf("expected image map in values")
	}
	if got := image["tag"]; got != "1.2.3" {
		t.Fatalf("expected tag fallback to version, got %v", got)
	}
}
