package kro

import (
	"testing"

	"github.com/jayadeyemi/ack-kro-gen/internal/config"
)

func TestCtrlSchemaDefaults(t *testing.T) {
	gs := config.GraphSpec{
		Service: "s3",
		Version: "1.2.3",
	}

	schema := CtrlSchema(gs, "S3")

	if got := schema.Spec.Name; got != "string | default=ack-s3-controller" {
		t.Fatalf("expected release name default, got %q", got)
	}
	if got := schema.Spec.Namespace; got != "string | default=ack-system" {
		t.Fatalf("expected namespace default, got %q", got)
	}
}
