package kro_test

import (
	"testing"

	"github.com/jayadeyemi/ack-kro-gen/internal/config"
	"github.com/jayadeyemi/ack-kro-gen/internal/kro"
)

func TestCtrlSchemaDefaultsNameAndNamespace(t *testing.T) {
	gs := config.GraphSpec{
		Service: "s3",
		Version: "v0.1.0",
	}

	schema := kro.CtrlSchema(gs, "S3")
	if got, want := schema.Spec.Name, "string | default=ack-s3-controller"; got != want {
		t.Fatalf("schema.Spec.Name = %q, want %q", got, want)
	}
	if got, want := schema.Spec.Namespace, "string | default=ack-system"; got != want {
		t.Fatalf("schema.Spec.Namespace = %q, want %q", got, want)
	}
}
